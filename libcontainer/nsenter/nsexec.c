
#define _GNU_SOURCE
#include <endian.h>
#include <errno.h>
#include <fcntl.h>
#include <grp.h>
#include <sched.h>
#include <setjmp.h>
#include <signal.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>
#include <unistd.h>

#include <sys/ioctl.h>
#include <sys/prctl.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/wait.h>

#include <linux/limits.h>
#include <linux/netlink.h>
#include <linux/types.h>

#include "getenv.h"
#include "log.h"
/* Get all of the CLONE_NEW* flags. */
#include "namespace.h"

/* Synchronisation values. */
enum sync_t {
	SYNC_USERMAP_PLS = 0x40,	/* Request parent to map our users. */
	SYNC_USERMAP_ACK = 0x41,	/* Mapping finished by the parent. */
	SYNC_RECVPID_PLS = 0x42,	/* Tell parent we're sending the PID. */
	SYNC_RECVPID_ACK = 0x43,	/* PID was correctly received by parent. */
	SYNC_TIMEOFFSETS_PLS = 0x44,	/* Request parent to write timens offsets. */
	SYNC_TIMEOFFSETS_ACK = 0x45,	/* Timens offsets were written. */
};

#define STAGE_SETUP  -1
/* longjmp() arguments. */
#define STAGE_CHILD   0
#define STAGE_INIT    1

/* Stores the current stage of nsexec. */
int current_stage = STAGE_SETUP;

/* Assume the stack grows down, so arguments should be above it. */
struct clone_t {
	/*
	 * Reserve some space for clone() to locate arguments
	 * and retcode in this place
	 */
	char stack[4096] __attribute__((aligned(16)));
	char stack_ptr[0];

	/* There's two children. This is used to execute the different code. */
	jmp_buf *env;
	int jmpval;
};

struct nlconfig_t {
	char *data;

	/* Process settings. */
	uint32_t cloneflags;
	char *oom_score_adj;
	size_t oom_score_adj_len;

	/* User namespace settings. */
	char *uidmap;
	size_t uidmap_len;
	char *gidmap;
	size_t gidmap_len;
	char *namespaces;
	size_t namespaces_len;
	uint8_t is_setgroup;

	/* Rootless container settings. */
	uint8_t is_rootless_euid;	/* boolean */
	char *uidmappath;
	size_t uidmappath_len;
	char *gidmappath;
	size_t gidmappath_len;

	/* Time NS offsets. */
	char *timensoffset;
	size_t timensoffset_len;
};

/*
 * List of netlink message types sent to us as part of bootstrapping the init.
 * These constants are defined in libcontainer/message_linux.go.
 */
#define INIT_MSG		62000
#define CLONE_FLAGS_ATTR	27281
#define NS_PATHS_ATTR		27282
#define OOM_SCORE_ADJ_ATTR	27286
#define TIMENSOFFSET_ATTR	27290

/*
 * Use the raw syscall for versions of glibc which don't include a function for
 * it, namely (glibc 2.12).
 */
#if __GLIBC__ == 2 && __GLIBC_MINOR__ < 14
#  define _GNU_SOURCE
#  include "syscall.h"
#  if !defined(SYS_setns) && defined(__NR_setns)
#    define SYS_setns __NR_setns
#  endif

#  ifndef SYS_setns
#    error "setns(2) syscall not supported by glibc version"
#  endif

int setns(int fd, int nstype)
{
	return syscall(SYS_setns, fd, nstype);
}
#endif

static int write_file(char *data, size_t data_len, char *pathfmt, ...)
{
	int fd, len, ret = 0;
	char path[PATH_MAX];

	va_list ap;
	va_start(ap, pathfmt);
	len = vsnprintf(path, PATH_MAX, pathfmt, ap);
	va_end(ap);
	if (len < 0)
		return -1;

	fd = open(path, O_RDWR);
	if (fd < 0) {
		return -1;
	}

	len = write(fd, data, data_len);
	if (len != data_len) {
		ret = -1;
		goto out;
	}

out:
	close(fd);
	return ret;
}

static void update_oom_score_adj(char *data, size_t len)
{
	if (data == NULL || len == 0)
		return;

	write_log(DEBUG, "update /proc/self/oom_score_adj to '%s'", data);
	if (write_file(data, len, "/proc/self/oom_score_adj") < 0)
		bail("failed to update /proc/self/oom_score_adj");
}

/* A dummy function that just jumps to the given jumpval. */
static int child_func(void *arg) __attribute__((noinline));
static int child_func(void *arg)
{
	struct clone_t *ca = (struct clone_t *)arg;
	longjmp(*ca->env, ca->jmpval);
}

static int clone_parent(jmp_buf *env, int jmpval) __attribute__((noinline));
static int clone_parent(jmp_buf *env, int jmpval)
{
	struct clone_t ca = {
		.env = env,
		.jmpval = jmpval,
	};

	return clone(child_func, ca.stack_ptr, CLONE_PARENT | SIGCHLD, &ca);
}

static uint32_t readint32(char *buf)
{
	return *(uint32_t *) buf;
}

static void nl_parse(int fd, struct nlconfig_t *config)
{
	size_t len, size;
	struct nlmsghdr hdr;
	char *data, *current;

	/* Retrieve the netlink header. */
	len = read(fd, &hdr, NLMSG_HDRLEN);
	if (len != NLMSG_HDRLEN)
		bail("invalid netlink header length %zu", len);

	if (hdr.nlmsg_type == NLMSG_ERROR)
		bail("failed to read netlink message");

	if (hdr.nlmsg_type != INIT_MSG)
		bail("unexpected msg type %d", hdr.nlmsg_type);

	/* Retrieve data. */
	size = NLMSG_PAYLOAD(&hdr, 0);
	current = data = malloc(size);
	if (!data)
		bail("failed to allocate %zu bytes of memory for nl_payload", size);

	len = read(fd, data, size);
	if (len != size)
		bail("failed to read netlink payload, %zu != %zu", len, size);

	/* Parse the netlink payload. */
	config->data = data;
	while (current < data + size) {
		struct nlattr *nlattr = (struct nlattr *)current;
		size_t payload_len = nlattr->nla_len - NLA_HDRLEN;

		/* Advance to payload. */
		current += NLA_HDRLEN;

		/* Handle payload. */
		switch (nlattr->nla_type) {
		case CLONE_FLAGS_ATTR:
			config->cloneflags = readint32(current);
			break;
		case OOM_SCORE_ADJ_ATTR:
			config->oom_score_adj = current;
			config->oom_score_adj_len = payload_len;
			break;
		case NS_PATHS_ATTR:
			config->namespaces = current;
			config->namespaces_len = payload_len;
			break;
		case TIMENSOFFSET_ATTR:
			config->timensoffset = current;
			config->timensoffset_len = payload_len;
			break;
		default:
			bail("unknown netlink message type %d", nlattr->nla_type);
		}

		current += NLA_ALIGN(payload_len);
	}
}

void nl_free(struct nlconfig_t *config)
{
	free(config->data);
}

struct namespace_t {
	int fd;
	char type[PATH_MAX];
	char path[PATH_MAX];
};

typedef int nsset_t;

static struct nstype_t {
	int type;
	char *name;
} all_ns_types[] = {
	{ CLONE_NEWCGROUP, "cgroup" },
	{ CLONE_NEWIPC, "ipc" },
	{ CLONE_NEWNS, "mnt" },
	{ CLONE_NEWNET, "net" },
	{ CLONE_NEWPID, "pid" },
	{ CLONE_NEWTIME, "time" },
	{ CLONE_NEWUSER, "user" },
	{ CLONE_NEWUTS, "uts" },
	{ },			/* null terminator */
};

/* Returns the clone(2) flag for a namespace, given the name of a namespace. */
static int nstype(char *name)
{
	for (struct nstype_t * ns = all_ns_types; ns->name != NULL; ns++)
		if (!strcmp(name, ns->name))
			return ns->type;
	/*
	 * setns(2) lets us join namespaces without knowing the type, but
	 * namespaces usually require special handling of some kind (so joining
	 * a namespace without knowing its type or joining a new namespace type
	 * without corresponding handling could result in broken behaviour) and
	 * the rest of runc doesn't allow unknown namespace types anyway.
	 */
	bail("unknown namespace type %s", name);
}

static nsset_t __open_namespaces(char *nsspec, struct namespace_t **ns_list, size_t *ns_len)
{
	int len = 0;
	nsset_t ns_to_join = 0;
	char *namespace, *saveptr = NULL;
	struct namespace_t *namespaces = NULL;

	namespace = strtok_r(nsspec, ",", &saveptr);

	if (!namespace || !strlen(namespace) || !strlen(nsspec))
		bail("ns paths are empty");

	do {
		int fd;
		char *path;
		struct namespace_t *ns;

		/* Resize the namespace array. */
		namespaces = realloc(namespaces, ++len * sizeof(struct namespace_t));
		if (!namespaces)
			bail("failed to reallocate namespace array");
		ns = &namespaces[len - 1];

		/* Split 'ns:path'. */
		path = strstr(namespace, ":");
		if (!path)
			bail("failed to parse %s", namespace);
		*path++ = '\0';

		fd = open(path, O_RDONLY);
		if (fd < 0)
			bail("failed to open %s", path);

		ns->fd = fd;
		strncpy(ns->type, namespace, PATH_MAX - 1);
		strncpy(ns->path, path, PATH_MAX - 1);
		ns->path[PATH_MAX - 1] = '\0';

		ns_to_join |= nstype(ns->type);
	} while ((namespace = strtok_r(NULL, ",", &saveptr)) != NULL);

	*ns_list = namespaces;
	*ns_len = len;
	return ns_to_join;
}

/*
 * Try to join all namespaces that are in the "allow" nsset, and return the
 * set we were able to successfully join. If a permission error is returned
 * from nsset(2), the namespace is skipped (non-permission errors are fatal).
 */
static nsset_t __join_namespaces(nsset_t allow, struct namespace_t *ns_list, size_t ns_len)
{
	nsset_t joined = 0;

	for (size_t i = 0; i < ns_len; i++) {
		struct namespace_t *ns = &ns_list[i];
		int type = nstype(ns->type);
		int err, saved_errno;

		if (!(type & allow))
			continue;

		err = setns(ns->fd, type);
		saved_errno = errno;
		write_log(DEBUG, "setns(%#x) into %s namespace (with path %s): %s",
			  type, ns->type, ns->path, strerror(errno));
		if (err < 0) {
			/* Skip permission errors. */
			if (saved_errno == EPERM)
				continue;
			bail("failed to setns into %s namespace", ns->type);
		}
		joined |= type;

		/*
		 * If we change user namespaces, make sure we switch to root in the
		 * namespace (this matches the logic for unshare(CLONE_NEWUSER)), lots
		 * of things can break if we aren't the right user. See
		 * <https://github.com/opencontainers/runc/issues/4466> for one example.
		 */
		if (type == CLONE_NEWUSER) {
			if (setresuid(0, 0, 0) < 0)
				bail("failed to become root in user namespace");
		}

		close(ns->fd);
		ns->fd = -1;
	}
	return joined;
}

static char *strappend(char *dst, char *src)
{
	if (!dst)
		return strdup(src);

	size_t len = strlen(dst) + strlen(src) + 1;
	dst = realloc(dst, len);
	strncat(dst, src, len);
	return dst;
}

static char *nsset_to_str(nsset_t nsset)
{
	char *str = NULL;
	for (struct nstype_t * ns = all_ns_types; ns->name != NULL; ns++) {
		if (ns->type & nsset) {
			if (str)
				str = strappend(str, ", ");
			str = strappend(str, ns->name);
		}
	}
	return str ? : strdup("");
}

static void __close_namespaces(nsset_t to_join, nsset_t joined, struct namespace_t *ns_list, size_t ns_len)
{
	/* We expect to have joined every namespace. */
	nsset_t failed_to_join = to_join & ~joined;

	/* Double-check that we used up (and thus joined) all of the nsfds. */
	for (size_t i = 0; i < ns_len; i++) {
		struct namespace_t *ns = &ns_list[i];
		int type = nstype(ns->type);

		if (ns->fd < 0)
			continue;

		failed_to_join |= type;
		write_log(FATAL, "failed to setns(%#x) into %s namespace (with path %s): %s",
			  type, ns->type, ns->path, strerror(EPERM));
		close(ns->fd);
		ns->fd = -1;
	}

	/* Make sure we joined the namespaces we planned to. */
	if (failed_to_join)
		bail("failed to join {%s} namespaces: %s", nsset_to_str(failed_to_join), strerror(EPERM));

	free(ns_list);
}

void join_namespaces(char *nsspec)
{
	nsset_t to_join = 0, joined = 0;
	struct namespace_t *ns_list;
	size_t ns_len;

	/*
	 * We have to open the file descriptors first, since after we join the
	 * mnt or user namespaces we might no longer be able to access the
	 * paths.
	 */
	to_join = __open_namespaces(nsspec, &ns_list, &ns_len);

	/*
	 * We first try to join all non-userns namespaces to join any namespaces
	 * that we might not be able to join once we switch credentials to the
	 * container's userns. We then join the user namespace, and then try to
	 * join any remaining namespaces (this last step is needed for rootless
	 * containers -- we don't get setns(2) permissions until we join the userns
	 * and get CAP_SYS_ADMIN).
	 *
	 * Splitting the joins this way is necessary for containers that are
	 * configured to join some externally-created namespace but are also
	 * configured to join an unrelated user namespace.
	 *
	 * This is similar to what nsenter(1) seems to do in practice.
	 */
	joined |= __join_namespaces(to_join & ~(joined | CLONE_NEWUSER), ns_list, ns_len);
	joined |= __join_namespaces(CLONE_NEWUSER, ns_list, ns_len);
	joined |= __join_namespaces(to_join & ~(joined | CLONE_NEWUSER), ns_list, ns_len);

	/* Verify that we joined all of the namespaces. */
	__close_namespaces(to_join, joined, ns_list, ns_len);
}

static inline int sane_kill(pid_t pid, int signum)
{
	if (pid > 0)
		return kill(pid, signum);
	else
		return 0;
}

void try_unshare(int flags, const char *msg)
{
	write_log(DEBUG, "unshare %s", msg);
	/*
	 * Kernels prior to v4.3 may return EINVAL on unshare when another process
	 * reads runc's /proc/$PID/status or /proc/$PID/maps. To work around this,
	 * retry on EINVAL a few times.
	 */
	int retries = 5;
	for (; retries > 0; retries--) {
		if (unshare(flags) == 0) {
			return;
		}
		if (errno != EINVAL)
			break;
	}
	bail("failed to unshare %s", msg);
}

void nsexec(void)
{
	int pipenum, syncfd;
	jmp_buf env;
	struct nlconfig_t config = { 0 };

	/*
	 * Setup a pipe to send logs to the parent. This should happen
	 * first, because bail will use that pipe.
	 */
	setup_logpipe();

	/*
	 * Get the init pipe fd from the environment. The init pipe is used to
	 * read the bootstrap data and tell the parent what the new pids are
	 * after the setup is done.
	 */
	pipenum = getenv_int("_LIBCONTAINER_INITPIPE");
	if (pipenum < 0) {
		/* We are not a runc init. Just return to go runtime. */
		return;
	}

	/*
	 * Get the stage1 pipe fd from the environment. The stage1 pipe is used to
	 * request the parent to do some operations that can't be done in the
	 * child process.
	 */
	syncfd = getenv_int("_LIBCONTAINER_STAGE1PIPE");
	if (syncfd < 0) {
		/* We are not a runc init. Just return to go runtime. */
		return;
	}

	write_log(DEBUG, "=> nsexec container setup");

	/* Parse all of the netlink configuration. */
	nl_parse(pipenum, &config);

	/* Set oom_score_adj. This has to be done before !dumpable because
	 * /proc/self/oom_score_adj is not writeable unless you're an privileged
	 * user (if !dumpable is set). All children inherit their parent's
	 * oom_score_adj value on fork(2) so this will always be propagated
	 * properly.
	 */
	update_oom_score_adj(config.oom_score_adj, config.oom_score_adj_len);

	/*
	 * Make the process non-dumpable, to avoid various race conditions that
	 * could cause processes in namespaces we're joining to access host
	 * resources (or potentially execute code).
	 *
	 * However, if the number of namespaces we are joining is 0, we are not
	 * going to be switching to a different security context. Thus setting
	 * ourselves to be non-dumpable only breaks things (like rootless
	 * containers), which is the recommendation from the kernel folks.
	 */
	if (config.namespaces) {
		write_log(DEBUG, "set process as non-dumpable");
		if (prctl(PR_SET_DUMPABLE, 0, 0, 0, 0) < 0)
			bail("failed to set process as non-dumpable");
	}

	/* TODO: Currently we aren't dealing with child deaths properly. */

	/*
	 * Okay, so this is quite annoying.
	 *
	 * In order for this unsharing code to be more extensible we need to split
	 * up unshare(CLONE_NEWUSER) and clone() in various ways. The ideal case
	 * would be if we did clone(CLONE_NEWUSER) and the other namespaces
	 * separately, but because of SELinux issues we cannot really do that. But
	 * we cannot just dump the namespace flags into clone(...) because several
	 * usecases (such as rootless containers) require more granularity around
	 * the namespace setup. In addition, some older kernels had issues where
	 * CLONE_NEWUSER wasn't handled before other namespaces (but we cannot
	 * handle this while also dealing with SELinux so we choose SELinux support
	 * over broken kernel support).
	 *
	 * However, if we unshare(2) the user namespace *before* we clone(2), then
	 * all hell breaks loose.
	 *
	 * The parent no longer has permissions to do many things (unshare(2) drops
	 * all capabilities in your old namespace), and the container cannot be set
	 * up to have more than one {uid,gid} mapping. This is obviously less than
	 * ideal. In order to fix this, we have to first clone(2) and then unshare.
	 *
	 * Unfortunately, it's not as simple as that. We have to fork to enter the
	 * PID namespace (the PID namespace only applies to children). Since we'll
	 * have to double-fork, this clone_parent() call won't be able to get the
	 * PID of the _actual_ init process (without doing more synchronisation than
	 * I can deal with at the moment). So we'll just get the parent to send it
	 * for us, the only job of this process is to update
	 * /proc/pid/{setgroups,uid_map,gid_map}.
	 *
	 * And as a result of the above, we also need to setns(2) in the first child
	 * because if we join a PID namespace in the topmost parent then our child
	 * will be in that namespace (and it will not be able to give us a PID value
	 * that makes sense without resorting to sending things with cmsg).
	 *
	 * This also deals with an older issue caused by dumping cloneflags into
	 * clone(2): On old kernels, CLONE_PARENT didn't work with CLONE_NEWPID, so
	 * we have to unshare(2) before clone(2) in order to do this. This was fixed
	 * in upstream commit 1f7f4dde5c945f41a7abc2285be43d918029ecc5, and was
	 * introduced by 40a0d32d1eaffe6aac7324ca92604b6b3977eb0e. As far as we're
	 * aware, the last mainline kernel which had this bug was Linux 3.12.
	 * However, we cannot comment on which kernels the broken patch was
	 * backported to.
	 *
	 * -- Aleksa "what has my life come to?" Sarai
	 */

	switch (setjmp(env)) {
		/*
		 * Stage 1: We're in the first child process. Our job is to join any
		 *          provided namespaces in the netlink payload and unshare all of
		 *          the requested namespaces. If we've been asked to CLONE_NEWUSER,
		 *          we will ask our parent (stage 0) to set up our user mappings
		 *          for us. Then, we create a new child (stage 2: STAGE_INIT) for
		 *          PID namespace. We then send the child's PID to our parent
		 *          (stage 0).
		 */
	case STAGE_CHILD:{
			pid_t stage2_pid = -1;
			enum sync_t s;

			/* For debugging. */
			current_stage = STAGE_CHILD;

			/* For debugging. */
			prctl(PR_SET_NAME, (unsigned long)"runc:[1:CHILD]", 0, 0, 0);
			write_log(DEBUG, "~> nsexec stage-1");

			/*
			 * We need to setns first. We cannot do this earlier (in stage 0)
			 * because of the fact that we forked to get here (the PID of
			 * [stage 2: STAGE_INIT]) would be meaningless). We could send it
			 * using cmsg(3) but that's just annoying.
			 */
			if (config.namespaces)
				join_namespaces(config.namespaces);

			/*
			 * Deal with user namespaces first. They are quite special, as they
			 * affect our ability to unshare other namespaces and are used as
			 * context for privilege checks.
			 *
			 * We don't unshare all namespaces in one go. The reason for this
			 * is that, while the kernel documentation may claim otherwise,
			 * there are certain cases where unsharing all namespaces at once
			 * will result in namespace objects being owned incorrectly.
			 * Ideally we should just fix these kernel bugs, but it's better to
			 * be safe than sorry, and fix them separately.
			 *
			 * A specific case of this is that the SELinux label of the
			 * internal kern-mount that mqueue uses will be incorrect if the
			 * UTS namespace is cloned before the USER namespace is mapped.
			 * I've also heard of similar problems with the network namespace
			 * in some scenarios. This also mirrors how LXC deals with this
			 * problem.
			 */
			if (config.cloneflags & CLONE_NEWUSER) {
				try_unshare(CLONE_NEWUSER, "user namespace");
				config.cloneflags &= ~CLONE_NEWUSER;

				/*
				 * We need to set ourselves as dumpable temporarily so that the
				 * parent process can write to our procfs files.
				 */
				if (config.namespaces) {
					write_log(DEBUG, "temporarily set process as dumpable");
					if (prctl(PR_SET_DUMPABLE, 1, 0, 0, 0) < 0)
						bail("failed to temporarily set process as dumpable");
				}

				/*
				 * We don't have the privileges to do any mapping here (see the
				 * clone_parent rant). So signal stage-0 to do the mapping for
				 * us.
				 */
				write_log(DEBUG, "request stage-0 to map user namespace");
				s = SYNC_USERMAP_PLS;
				if (write(syncfd, &s, sizeof(s)) != sizeof(s))
					bail("failed to sync with parent: write(SYNC_USERMAP_PLS)");

				/* ... wait for mapping ... */
				write_log(DEBUG, "waiting stage-0 to complete the mapping of user namespace");
				if (read(syncfd, &s, sizeof(s)) != sizeof(s))
					bail("failed to sync with parent: read(SYNC_USERMAP_ACK) got %d", s);
				if (s != SYNC_USERMAP_ACK)
					bail("failed to sync with parent: SYNC_USERMAP_ACK");

				/* Revert temporary re-dumpable setting. */
				if (config.namespaces) {
					write_log(DEBUG, "re-set process as non-dumpable");
					if (prctl(PR_SET_DUMPABLE, 0, 0, 0, 0) < 0)
						bail("failed to re-set process as non-dumpable");
				}

				/* Become root in the namespace proper. */
				if (setresuid(0, 0, 0) < 0)
					bail("failed to become root in user namespace");
			}

			/*
			 * Unshare all of the namespaces. Now, it should be noted that this
			 * ordering might break in the future (especially with rootless
			 * containers). But for now, it's not possible to split this into
			 * CLONE_NEWUSER + [the rest] because of some RHEL SELinux issues.
			 *
			 * Note that we don't merge this with clone() because there were
			 * some old kernel versions where clone(CLONE_PARENT | CLONE_NEWPID)
			 * was broken, so we'll just do it the long way anyway.
			 */
			try_unshare(config.cloneflags, "remaining namespaces");

			if (config.timensoffset) {
				write_log(DEBUG, "request stage-0 to write timens offsets");

				s = SYNC_TIMEOFFSETS_PLS;
				if (write(syncfd, &s, sizeof(s)) != sizeof(s))
					bail("failed to sync with parent: write(SYNC_TIMEOFFSETS_PLS)");

				if (read(syncfd, &s, sizeof(s)) != sizeof(s))
					bail("failed to sync with parent: read(SYNC_TIMEOFFSETS_ACK)");
				if (s != SYNC_TIMEOFFSETS_ACK)
					bail("failed to sync with parent: SYNC_TIMEOFFSETS_ACK: got %u", s);
			}

			/*
			 * TODO: What about non-namespace clone flags that we're dropping here?
			 *
			 * We fork again because of PID namespace, setns(2) or unshare(2) don't
			 * change the PID namespace of the calling process, because doing so
			 * would change the caller's idea of its own PID (as reported by getpid()),
			 * which would break many applications and libraries, so we must fork
			 * to actually enter the new PID namespace.
			 */
			write_log(DEBUG, "spawn stage-2");
			stage2_pid = clone_parent(&env, STAGE_INIT);
			if (stage2_pid < 0)
				bail("unable to spawn stage-2");

			/* Send the child to our parent, which knows what it's doing. */
			write_log(DEBUG, "request stage-0 to forward stage-2 pid (%d)", stage2_pid);
			s = SYNC_RECVPID_PLS;
			if (write(syncfd, &s, sizeof(s)) != sizeof(s)) {
				sane_kill(stage2_pid, SIGKILL);
				bail("failed to sync with parent: write(SYNC_RECVPID_PLS)");
			}
			if (write(syncfd, &stage2_pid, sizeof(stage2_pid)) != sizeof(stage2_pid)) {
				sane_kill(stage2_pid, SIGKILL);
				bail("failed to sync with parent: write(stage2_pid)");
			}

			/* ... wait for parent to get the pid ... */
			if (read(syncfd, &s, sizeof(s)) != sizeof(s)) {
				sane_kill(stage2_pid, SIGKILL);
				bail("failed to sync with parent: read(SYNC_RECVPID_ACK)");
			}
			if (s != SYNC_RECVPID_ACK) {
				sane_kill(stage2_pid, SIGKILL);
				bail("failed to sync with parent: SYNC_RECVPID_ACK: got %u", s);
			}

			close(syncfd);

			/* Our work is done. [Stage 2: STAGE_INIT] is doing the rest of the work. */
			write_log(DEBUG, "<~ nsexec stage-1");
			exit(0);
		}
		break;

		/*
		 * Stage 2: We're the final child process, and the only process that will
		 *          actually return to the Go runtime. Our job is to just do the
		 *          final cleanup steps and then return to the Go runtime to allow
		 *          init_linux.go to run.
		 */
	case STAGE_INIT:{
			/* For debugging. */
			current_stage = STAGE_INIT;

			/* For debugging. */
			prctl(PR_SET_NAME, (unsigned long)"runc:[2:INIT]", 0, 0, 0);
			write_log(DEBUG, "~> nsexec stage-2");

			close(syncfd);

			/* Free netlink data. */
			nl_free(&config);

			/* Finish executing, let the Go runtime take over. */
			write_log(DEBUG, "<= nsexec container setup");
			write_log(DEBUG, "booting up go runtime ...");
			return;
		}
		break;
	default:
		bail("unexpected jump value");
	}

	/* Should never be reached. */
	bail("should never be reached");
}
