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

/* All arguments should be above stack, because it grows down */
struct clone_arg {
	/*
	 * Reserve some space for clone() to locate arguments
	 * and retcode in this place
	 */
	char stack[4096] __attribute__ ((aligned(8)));
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
	child = clone(child_func, ca.stack_ptr, CLONE_PARENT | SIGCHLD | flags, &ca);
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

// return 0 for big endian, 1 for little endian.
static int is_little_endian()
{
	int i = 0x01234567;
	return (*((uint8_t *) (&i))) == 0x67;
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

static uint32_t readint32(char *buf, int *start)
{
	union {
		uint32_t n;
		char arr[4];
	} num;
	int i = 0;
	for (i = 0; i < 4; i++) {
		num.arr[i] = buf[*start + i];
	}
	*start += 4;
	if (is_little_endian() == 1) {
		return le32toh(num.n);
	}
	return be32toh(num.n);
}

static uint16_t readint16(char *buf, int *start)
{
	union {
		uint16_t n;
		char arr[2];
	} num;
	int i = 0;
	for (i = 0; i < 2; i++) {
		num.arr[i] = buf[*start + i];
	}
	*start += 2;
	if (is_little_endian() == 1) {
		return le16toh(num.n);
	}
	return be16toh(num.n);
}

static uint8_t readint8(char *buf, int *start)
{
	union {
		uint8_t n;
		char arr[1];
	} num;
	num.arr[0] = buf[*start];
	*start += 1;
	return num.n;
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

const uint16_t RUNC_INIT = 62000;
// list of known message types we want to send to bootstrap program
const uint16_t RUNC_INIT_CLONEFLAGS = 27281;
const uint16_t RUNC_INIT_CONSOLEPATH = 27282;
const uint16_t RUNC_INIT_NSPATHS = 27283;
const uint16_t RUNC_INIT_UIDMAP = 27284;
const uint16_t RUNC_INIT_GIDMAP = 27285;
const uint16_t RUNC_INIT_SETGROUP = 27286;

void nsexec()
{
	jmp_buf env;

	// if we dont have init pipe, then just return to the parent
	int pipenum = get_init_pipe();
	if (pipenum == -1) {
		return;
	}

	int len;
	static char buf[16384];
	struct iovec iov = { buf, sizeof(buf) };
	struct msghdr msg;
	struct nlmsghdr *nh;

	memset(&msg, 0, sizeof(msg));
	msg.msg_name = &nh;
	msg.msg_namelen = sizeof(nh);
	msg.msg_iov = &iov;
	msg.msg_iovlen = 1;
	while (1) {
		len = recvmsg(pipenum, &msg, 0);
		if (len <= 0) {
			if (errno == EINTR) {
				continue;
			}
			pr_perror("invalid netlink init message size %d", len);
			exit(1);
		}
		break;
	}

	nh = (struct nlmsghdr *)buf;
	if (NLMSG_OK(nh, len) != 1) {
		pr_perror("malformed message");
		exit(1);
	};
	if (nh->nlmsg_type == NLMSG_ERROR) {
		pr_perror("failed to read netlink message");
		exit(1);
	}
	if (nh->nlmsg_type != RUNC_INIT) {
		pr_perror("unexpected msg type %d", nh->nlmsg_type);
		exit(1);
	}

	int total = NLMSG_PAYLOAD(nh, 0);
	char data[total];
	memcpy(data, NLMSG_DATA(nh), total);

	// processing message
	int start = 0;
	uint32_t cloneflags = -1;
	uint8_t is_setgroup = 0;
	int consolefd = -1;
	int uidmap_start = -1, uidmap_len = -1;
	int gidmap_start = -1, gidmap_len = -1;
	while (start < total) {
		uint16_t msgtype = readint16(data, &start);
		if (msgtype == RUNC_INIT_CLONEFLAGS) {
			cloneflags = readint32(data, &start);
		} else if (msgtype == RUNC_INIT_CONSOLEPATH) {
			uint32_t consolelen = readint32(data, &start);
			// get the console path before setns because it may change mnt namespace
			char console[consolelen + 1];
			strncpy(console, data + start, consolelen);
			console[consolelen] = '\0';
			consolefd = open(console, O_RDWR);
			if (consolefd < 0) {
				pr_perror("Failed to open console %s", console);
				exit(1);
			}
			start = start + consolelen;
		} else if (msgtype == RUNC_INIT_NSPATHS) {
			uint32_t nspaths_len = readint32(data, &start);
			char nspaths[nspaths_len + 1];
			strncpy(nspaths, data + start, nspaths_len);
			nspaths[nspaths_len] = '\0';
			// if custom namespaces are required, open all descriptors and perform
			// setns on them
			int nslen = num_namespaces(nspaths);
			int fds[nslen];
			char *nslist[nslen];
			int i = -1;
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
					pr_perror("Failed to setns to %s", nslist[i]);
					exit(1);
				}
				close(fds[i]);
			}
			start = start + nspaths_len;
		} else if (msgtype == RUNC_INIT_UIDMAP) {
			uidmap_len = readint32(data, &start);
			uidmap_start = start;
			start = start + uidmap_len;
		} else if (msgtype == RUNC_INIT_GIDMAP) {
			gidmap_len = readint32(data, &start);
			gidmap_start = start;
			start = start + gidmap_len;
		} else if (msgtype == RUNC_INIT_SETGROUP) {
			is_setgroup = readint8(data, &start);
		} else {
			pr_perror("unknown msgtype %d", msgtype);
			exit(1);
		}
	}

	// required clone_flags to be passed
	if (cloneflags == -1) {
		pr_perror("missing clone_flags");
		exit(1);
	}
	// prepare sync pipe between parent and child. We need this to let the child
	// know that the parent has finished setting up 
	int syncpipe[2];
	syncpipe[0] = -1;
	syncpipe[1] = -1;
	if (pipe(syncpipe) != 0) {
		pr_perror("failed to setup sync pipe between parent and child");
		exit(1);
	};

	if (setjmp(env) == 1) {
		// close the writing side of pipe
		close(syncpipe[1]);
		uint8_t s;
		if (read(syncpipe[0], &s, 1) != 1 || s != 1) {
			pr_perror("failed to read sync byte from parent");
			exit(1);
		};
		if (setsid() == -1) {
			pr_perror("setsid failed");
			exit(1);
		}
		// Child
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
	// if we specifies uid_map and gid_map, writes the data to /proc files
	if (uidmap_start > 0 && uidmap_len > 0) {
		char buf[PATH_MAX];
		if (snprintf(buf, sizeof(buf), "/proc/%d/uid_map", child) < 0) {
			pr_perror("failed to construct uid_map file for %d", child);
			exit(1);
		}
		int fd = open(buf, O_RDWR);
		writedata(fd, data, uidmap_start, uidmap_len);
	}
	if (gidmap_start > 0 && gidmap_len > 0) {
		if (is_setgroup == 1) {
			char buf[PATH_MAX];
			if (snprintf(buf, sizeof(buf), "/proc/%d/setgroups", child) < 0) {
				pr_perror("failed to construct setgroups file for %d", child);
				exit(1);
			}
			int fd = open(buf, O_RDWR);
			if (write(fd, "allow", 5) != 5) {
				// If the kernel is too old to support /proc/PID/setgroups,
				// write will return ENOENT; this is OK.
				if (errno != ENOENT) {
					pr_perror("failed to write allow to %s", buf);
					exit(1);
				}
			}
		}
		// write gid mappings
		char buf[PATH_MAX];
		if (snprintf(buf, sizeof(buf), "/proc/%d/gid_map", child) < 0) {
			pr_perror("failed to construct gid_map file for %d", child);
			exit(1);
		}
		int fd = open(buf, O_RDWR);
		writedata(fd, data, gidmap_start, gidmap_len);
	}
	close(syncpipe[0]);
	uint8_t s = 1;
	if (write(syncpipe[1], &s, 1) != 1) {
		pr_perror("failed to write sync byte to child");
		exit(1);
	};
	// parent to finish the bootstrap process
	char child_data[PATH_MAX];
	len = snprintf(child_data, sizeof(child_data), "{ \"pid\" : %d }\n", child);
	if (write(pipenum, child_data, len) != len) {
		pr_perror("Unable to send a child pid");
		kill(child, SIGKILL);
		exit(1);
	}
	exit(0);
}
