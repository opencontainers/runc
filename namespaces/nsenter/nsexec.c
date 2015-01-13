#include <stdlib.h>
#include <unistd.h>
#include <stdio.h>
#include <errno.h>
#include <string.h>

#include <linux/limits.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <signal.h>

// Use raw setns syscall for versions of glibc that don't include it (namely glibc-2.12)
#if __GLIBC__ == 2 && __GLIBC_MINOR__ < 14
#define _GNU_SOURCE
#include <sched.h>
#include "syscall.h"
#ifdef SYS_setns
int setns(int fd, int nstype)
{
	return syscall(SYS_setns, fd, nstype);
}
#endif
#endif

void nsexec()
{
	char *namespaces[] = { "ipc", "uts", "net", "pid", "mnt" };
	const int num = sizeof(namespaces) / sizeof(char *);
	char buf[PATH_MAX], *val;
	int child, i, tfd;
	pid_t pid;

	val = getenv("_LIBCONTAINER_INITPID");
	if (val == NULL)
		return;

	pid = atoi(val);
	snprintf(buf, sizeof(buf), "%d", pid);
	if (strcmp(val, buf)) {
		fprintf(stderr, "Unable to parse _LIBCONTAINER_INITPID");
		exit(1);
	}

	/* Check that the specified process exists */
	snprintf(buf, PATH_MAX - 1, "/proc/%d/ns", pid);
	tfd = open(buf, O_DIRECTORY | O_RDONLY);
	if (tfd == -1) {
		fprintf(stderr,
			"nsenter: Failed to open \"%s\" with error: \"%s\"\n",
			buf, strerror(errno));
		exit(1);
	}

	for (i = 0; i < num; i++) {
		struct stat st;
		int fd;

		/* Symlinks on all namespaces exist for dead processes, but they can't be opened */
		if (fstatat(tfd, namespaces[i], &st, AT_SYMLINK_NOFOLLOW) == -1) {
			// Ignore nonexistent namespaces.
			if (errno == ENOENT)
				continue;
		}

		fd = openat(tfd, namespaces[i], O_RDONLY);
		if (fd == -1) {
			fprintf(stderr,
				"nsenter: Failed to open ns file \"%s\" for ns \"%s\" with error: \"%s\"\n",
				buf, namespaces[i], strerror(errno));
			exit(1);
		}
		// Set the namespace.
		if (setns(fd, 0) == -1) {
			fprintf(stderr,
				"nsenter: Failed to setns for \"%s\" with error: \"%s\"\n",
				namespaces[i], strerror(errno));
			exit(1);
		}
		close(fd);
	}

	child = fork();
	if (child < 0) {
		fprintf(stderr, "Unable to fork: %s", strerror(errno));
		exit(1);
	}
	// We must fork to actually enter the PID namespace.
	if (child == 0) {
		// Finish executing, let the Go runtime take over.
		return;
	} else {
		// Parent, wait for the child.
		int status = 0;
		if (waitpid(child, &status, 0) == -1) {
			fprintf(stderr,
				"nsenter: Failed to waitpid with error: \"%s\"\n",
				strerror(errno));
			exit(1);
		}
		// Forward the child's exit code or re-send its death signal.
		if (WIFEXITED(status)) {
			exit(WEXITSTATUS(status));
		} else if (WIFSIGNALED(status)) {
			kill(getpid(), WTERMSIG(status));
		}

		exit(1);
	}

	return;
}
