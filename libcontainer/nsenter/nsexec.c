#define _GNU_SOURCE
#include <endian.h>
#include <errno.h>
#include <fcntl.h>
#include <linux/limits.h>
#include <linux/netlink.h>
#include <sched.h>
#include <setjmp.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <unistd.h>

#include <bits/sockaddr.h>
#include <linux/netlink.h>
#include <linux/types.h>
#include <stdint.h>
#include <sys/socket.h>

/* All arguments should be above stack, because it grows down */
struct clone_arg {
	/*
	 * Reserve some space for clone() to locate arguments
	 * and retcode in this place
	 */
	char stack[4096] __attribute__ ((aligned(16)));
	char stack_ptr[0];
	jmp_buf *env;
};

#define pr_perror(fmt, ...) fprintf(stderr, "nsenter: " fmt ": %m\n", ##__VA_ARGS__)

static int child_func(void *_arg)
{
	struct clone_arg *arg = (struct clone_arg *)_arg;
	longjmp(*arg->env, 1);
}

// Use raw setns syscall for versions of glibc that don't include it (namely glibc-2.12)
#if __GLIBC__ == 2 && __GLIBC_MINOR__ < 14
#define _GNU_SOURCE
#include "syscall.h"
#if defined(__NR_setns) && !defined(SYS_setns)
#define SYS_setns __NR_setns
#endif
#ifdef SYS_setns
int setns(int fd, int nstype)
{
	return syscall(SYS_setns, fd, nstype);
}
#endif
#endif

static int clone_parent(jmp_buf * env, int flags) __attribute__ ((noinline));
static int clone_parent(jmp_buf * env, int flags)
{
	struct clone_arg ca;
	int child;

	ca.env = env;
	child =
	    clone(child_func, ca.stack_ptr, CLONE_PARENT | SIGCHLD | flags,
		  &ca);
	return child;
}

// get init pipe from the parent. It's used to read bootstrap data, and to
// write pid to after nsexec finishes setting up the environment.
static int get_init_pipe()
{
	char buf[PATH_MAX], *initpipe;
	int pipenum = -1;

	initpipe = getenv("_LIBCONTAINER_INITPIPE");
	if (initpipe == NULL) {
		return -1;
	}

	pipenum = atoi(initpipe);
	snprintf(buf, sizeof(buf), "%d", pipenum);
	if (strcmp(initpipe, buf)) {
		pr_perror("Unable to parse _LIBCONTAINER_INITPIPE");
		exit(1);
	}

	return pipenum;
}

// num_namespaces returns the number of additional namespaces to setns. The
// argument is a comma-separated string of namespace paths.
static int num_namespaces(char *nspaths)
{
	int size = 0, i = 0;

	for (i = 0; nspaths[i]; i++) {
		if (nspaths[i] == ',') {
			size += 1;
		}
	}

	return size + 1;
}

static uint32_t readint32(char *buf)
{
	return *(uint32_t *) buf;
}

static uint8_t readint8(char *buf)
{
	return *(uint8_t *) buf;
}

static void writedata(int fd, char *data, int start, int len)
{
	int written = 0;
	while (written < len) {
		size_t nbyte, i;
		if ((len - written) < 1024) {
			nbyte = len - written;
		} else {
			nbyte = 1024;
		}
		i = write(fd, data + start + written, nbyte);
		if (i == -1) {
			pr_perror("failed to write data to %d", fd);
			exit(1);
		}
		written += i;
	}
}

// list of known message types we want to send to bootstrap program
// These are defined in libcontainer/message_linux.go
#define INIT_MSG            62000
#define CLONE_FLAGS_ATTR    27281
#define CONSOLE_PATH_ATTR   27282
#define NS_PATHS_ATTR       27283
#define UIDMAP_ATTR         27284
#define GIDMAP_ATTR         27285
#define SETGROUP_ATTR       27286

void nsexec()
{
	jmp_buf env;
	int pipenum;

	// if we dont have init pipe, then just return to the parent
	pipenum = get_init_pipe();
	if (pipenum == -1) {
		return;
	}
	// Retrieve the netlink header
	struct nlmsghdr nl_msg_hdr;
	int len;

	if ((len = read(pipenum, &nl_msg_hdr, NLMSG_HDRLEN)) != NLMSG_HDRLEN) {
		pr_perror("Failed to read netlink header, got %d instead of %d",
			  len, NLMSG_HDRLEN);
		exit(1);
	}

	if (nl_msg_hdr.nlmsg_type == NLMSG_ERROR) {
		pr_perror("failed to read netlink message");
		exit(1);
	}
	if (nl_msg_hdr.nlmsg_type != INIT_MSG) {
		pr_perror("unexpected msg type %d", nl_msg_hdr.nlmsg_type);
		exit(1);
	}
	// Retrieve data
	int nl_total_size = NLMSG_PAYLOAD(&nl_msg_hdr, 0);
	char data[nl_total_size];

	if ((len = read(pipenum, data, nl_total_size)) != nl_total_size) {
		pr_perror
		    ("Failed to read netlink payload, got %d instead of %d",
		     len, nl_total_size);
		exit(1);
	}
	// Process the passed attributes
	int start = 0;
	uint32_t cloneflags = -1;
	uint8_t is_setgroup = 0;
	int consolefd = -1;
	int uidmap_start = -1, uidmap_len = -1;
	int gidmap_start = -1, gidmap_len = -1;
	int payload_len;
	struct nlattr *nlattr;

	while (start < nl_total_size) {
		nlattr = (struct nlattr *)(data + start);
		start += NLA_HDRLEN;
		payload_len = nlattr->nla_len - NLA_HDRLEN;

		if (nlattr->nla_type == CLONE_FLAGS_ATTR) {
			cloneflags = readint32(data + start);
		} else if (nlattr->nla_type == CONSOLE_PATH_ATTR) {
			// get the console path before setns because it may change mnt namespace
			consolefd = open(data + start, O_RDWR);
			if (consolefd < 0) {
				pr_perror("Failed to open console %s",
					  data + start);
				exit(1);
			}
		} else if (nlattr->nla_type == NS_PATHS_ATTR) {
			char nspaths[payload_len + 1];

			strncpy(nspaths, data + start, payload_len);
			nspaths[payload_len] = '\0';

			// if custom namespaces are required, open all descriptors and perform
			// setns on them
			int nslen = num_namespaces(nspaths);
			int fds[nslen];
			char *nslist[nslen];
			int i;
			char *ns, *saveptr;

			for (i = 0; i < nslen; i++) {
				char *str = NULL;

				if (i == 0) {
					str = nspaths;
				}
				ns = strtok_r(str, ",", &saveptr);
				if (ns == NULL) {
					break;
				}
				fds[i] = open(ns, O_RDONLY);
				if (fds[i] == -1) {
					pr_perror("Failed to open %s", ns);
					exit(1);
				}
				nslist[i] = ns;
			}

			for (i = 0; i < nslen; i++) {
				if (setns(fds[i], 0) != 0) {
					pr_perror("Failed to setns to %s",
						  nslist[i]);
					exit(1);
				}
				close(fds[i]);
			}
		} else if (nlattr->nla_type == UIDMAP_ATTR) {
			uidmap_len = payload_len;
			uidmap_start = start;
		} else if (nlattr->nla_type == GIDMAP_ATTR) {
			gidmap_len = payload_len;
			gidmap_start = start;
		} else if (nlattr->nla_type == SETGROUP_ATTR) {
			is_setgroup = readint8(data + start);
		} else {
			pr_perror("unknown netlink message type %d",
				  nlattr->nla_type);
			exit(1);
		}

		start += NLA_ALIGN(payload_len);
	}

	// required clone_flags to be passed
	if (cloneflags == -1) {
		pr_perror("missing clone_flags");
		exit(1);
	}
	// prepare sync pipe between parent and child. We need this to let the child
	// know that the parent has finished setting up
	int syncpipe[2] = { -1, -1 };
	if (pipe(syncpipe) != 0) {
		pr_perror("failed to setup sync pipe between parent and child");
		exit(1);
	};

	if (setjmp(env) == 1) {
		// Child
		uint8_t s;

		// close the writing side of pipe
		close(syncpipe[1]);

		// sync with parent
		if (read(syncpipe[0], &s, 1) != 1 || s != 1) {
			pr_perror("failed to read sync byte from parent");
			exit(1);
		};

		if (setsid() == -1) {
			pr_perror("setsid failed");
			exit(1);
		}

		if (consolefd != -1) {
			if (ioctl(consolefd, TIOCSCTTY, 0) == -1) {
				pr_perror("ioctl TIOCSCTTY failed");
				exit(1);
			}
			if (dup3(consolefd, STDIN_FILENO, 0) != STDIN_FILENO) {
				pr_perror("Failed to dup 0");
				exit(1);
			}
			if (dup3(consolefd, STDOUT_FILENO, 0) != STDOUT_FILENO) {
				pr_perror("Failed to dup 1");
				exit(1);
			}
			if (dup3(consolefd, STDERR_FILENO, 0) != STDERR_FILENO) {
				pr_perror("Failed to dup 2");
				exit(1);
			}
		}
		// Finish executing, let the Go runtime take over.
		return;
	}
	// Parent

	// We must fork to actually enter the PID namespace, use CLONE_PARENT
	// so the child can have the right parent, and we don't need to forward
	// the child's exit code or resend its death signal.
	int child = clone_parent(&env, cloneflags);
	if (child < 0) {
		pr_perror("Unable to fork");
		exit(1);
	}
	// if uid_map and gid_map were specified, writes the data to /proc files
	if (uidmap_start > 0 && uidmap_len > 0) {
		char buf[PATH_MAX];
		if (snprintf(buf, sizeof(buf), "/proc/%d/uid_map", child) < 0) {
			pr_perror("failed to construct uid_map file for %d",
				  child);
			exit(1);
		}

		int fd = open(buf, O_RDWR);
		writedata(fd, data, uidmap_start, uidmap_len);
	}

	if (gidmap_start > 0 && gidmap_len > 0) {
		if (is_setgroup == 1) {
			char buf[PATH_MAX];
			if (snprintf
			    (buf, sizeof(buf), "/proc/%d/setgroups",
			     child) < 0) {
				pr_perror
				    ("failed to construct setgroups file for %d",
				     child);
				exit(1);
			}

			int fd = open(buf, O_RDWR);
			if (write(fd, "allow", 5) != 5) {
				// If the kernel is too old to support /proc/PID/setgroups,
				// write will return ENOENT; this is OK.
				if (errno != ENOENT) {
					pr_perror("failed to write allow to %s",
						  buf);
					exit(1);
				}
			}
		}
		// write gid mappings
		char buf[PATH_MAX];
		if (snprintf(buf, sizeof(buf), "/proc/%d/gid_map", child) < 0) {
			pr_perror("failed to construct gid_map file for %d",
				  child);
			exit(1);
		}

		int fd = open(buf, O_RDWR);
		writedata(fd, data, gidmap_start, gidmap_len);
	}
	// Send the sync signal to the child
	close(syncpipe[0]);
	uint8_t s = 1;
	if (write(syncpipe[1], &s, 1) != 1) {
		pr_perror("failed to write sync byte to child");
		exit(1);
	};

	// parent to finish the bootstrap process
	char child_data[PATH_MAX];
	len =
	    snprintf(child_data, sizeof(child_data), "{ \"pid\" : %d }\n",
		     child);
	if (write(pipenum, child_data, len) != len) {
		pr_perror("Unable to send a child pid");
		kill(child, SIGKILL);
		exit(1);
	}
	exit(0);
}
