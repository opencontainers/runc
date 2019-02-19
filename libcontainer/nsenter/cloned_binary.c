/*
 * Copyright (C) 2019 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2019 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#define _GNU_SOURCE
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>
#include <limits.h>
#include <fcntl.h>
#include <errno.h>

#include <sys/types.h>
#include <sys/stat.h>
#include <sys/vfs.h>
#include <sys/mman.h>
#include <sys/sendfile.h>
#include <sys/syscall.h>

/* Use our own wrapper for memfd_create. */
#if !defined(SYS_memfd_create) && defined(__NR_memfd_create)
#  define SYS_memfd_create __NR_memfd_create
#endif
/* memfd_create(2) flags -- copied from <linux/memfd.h>. */
#ifndef MFD_CLOEXEC
#  define MFD_CLOEXEC       0x0001U
#  define MFD_ALLOW_SEALING 0x0002U
#endif
int memfd_create(const char *name, unsigned int flags)
{
#ifdef SYS_memfd_create
	return syscall(SYS_memfd_create, name, flags);
#else
	errno = ENOSYS;
	return -1;
#endif
}


/* This comes directly from <linux/fcntl.h>. */
#ifndef F_LINUX_SPECIFIC_BASE
#  define F_LINUX_SPECIFIC_BASE 1024
#endif
#ifndef F_ADD_SEALS
#  define F_ADD_SEALS (F_LINUX_SPECIFIC_BASE + 9)
#  define F_GET_SEALS (F_LINUX_SPECIFIC_BASE + 10)
#endif
#ifndef F_SEAL_SEAL
#  define F_SEAL_SEAL   0x0001	/* prevent further seals from being set */
#  define F_SEAL_SHRINK 0x0002	/* prevent file from shrinking */
#  define F_SEAL_GROW   0x0004	/* prevent file from growing */
#  define F_SEAL_WRITE  0x0008	/* prevent writes */
#endif

#define RUNC_MEMFD_COMMENT "runc_cloned:/proc/self/exe"
#define RUNC_MEMFD_SEALS \
	(F_SEAL_SEAL | F_SEAL_SHRINK | F_SEAL_GROW | F_SEAL_WRITE)

static void *must_realloc(void *ptr, size_t size)
{
	void *old = ptr;
	do {
		ptr = realloc(old, size);
	} while(!ptr);
	return ptr;
}

/*
 * Verify whether we are currently in a self-cloned program (namely, is
 * /proc/self/exe a memfd). F_GET_SEALS will only succeed for memfds (or rather
 * for shmem files), and we want to be sure it's actually sealed.
 */
static int is_self_cloned(void)
{
	int fd, ret, is_cloned = 0;

	fd = open("/proc/self/exe", O_RDONLY|O_CLOEXEC);
	if (fd < 0)
		return -ENOTRECOVERABLE;

	/* First check memfd. */
	ret = fcntl(fd, F_GET_SEALS);
	if (ret >= 0) {
		is_cloned = (ret == RUNC_MEMFD_SEALS);
	} else {
		/*
		 * Okay, we're a tmpfile -- or we're currently running on RHEL <=7.6
		 * which appears to have a borked backport of F_GET_SEALS. Either way,
		 * having a file which has no hardlinks indicates that we aren't using
		 * a host-side "runc" binary and this is something that a container
		 * cannot fake (because unlinking requires being able to resolve the
		 * path that you want to unlink).
		 */
		struct stat statbuf = {};
		if (fstat(fd, &statbuf) >= 0)
			is_cloned = (statbuf.st_nlink == 0);
	}

	close(fd);
	return is_cloned;
}

/* Read a given file into a new buffer, and providing the length. */
static char *read_file(char *path, size_t *length)
{
	int fd;
	char buf[4096], *copy = NULL;

	if (!length)
		return NULL;

	fd = open(path, O_RDONLY | O_CLOEXEC);
	if (fd < 0)
		return NULL;

	*length = 0;
	for (;;) {
		int n;

		n = read(fd, buf, sizeof(buf));
		if (n < 0)
			goto error;
		if (!n)
			break;

		copy = must_realloc(copy, (*length + n) * sizeof(*copy));
		memcpy(copy + *length, buf, n);
		*length += n;
	}
	close(fd);
	return copy;

error:
	close(fd);
	free(copy);
	return NULL;
}

/*
 * A poor-man's version of "xargs -0". Basically parses a given block of
 * NUL-delimited data, within the given length and adds a pointer to each entry
 * to the array of pointers.
 */
static int parse_xargs(char *data, int data_length, char ***output)
{
	int num = 0;
	char *cur = data;

	if (!data || *output != NULL)
		return -1;

	while (cur < data + data_length) {
		num++;
		*output = must_realloc(*output, (num + 1) * sizeof(**output));
		(*output)[num - 1] = cur;
		cur += strlen(cur) + 1;
	}
	(*output)[num] = NULL;
	return num;
}

/*
 * "Parse" out argv from /proc/self/cmdline.
 * This is necessary because we are running in a context where we don't have a
 * main() that we can just get the arguments from.
 */
static int fetchve(char ***argv)
{
	char *cmdline = NULL;
	size_t cmdline_size;

	cmdline = read_file("/proc/self/cmdline", &cmdline_size);
	if (!cmdline)
		goto error;

	if (parse_xargs(cmdline, cmdline_size, argv) <= 0)
		goto error;

	return 0;

error:
	free(cmdline);
	return -EINVAL;
}

enum {
	EFD_NONE = 0,
	EFD_MEMFD,
	EFD_FILE,
};

/*
 * This comes from <linux/fcntl.h>. We can't hard-code __O_TMPFILE because it
 * changes depending on the architecture. If we don't have O_TMPFILE we always
 * have the mkostemp(3) fallback.
 */
#ifndef O_TMPFILE
#  if defined(__O_TMPFILE) && defined(O_DIRECTORY)
#    define O_TMPFILE (__O_TMPFILE | O_DIRECTORY)
#  endif
#endif

static int make_execfd(int *fdtype)
{
	int fd = -1;
	char template[PATH_MAX] = {0};
	char *prefix = secure_getenv("_LIBCONTAINER_STATEDIR");

	if (!prefix || *prefix != '/')
		prefix = "/tmp";
	if (snprintf(template, sizeof(template), "%s/runc.XXXXXX", prefix) < 0)
		return -1;

	/*
	 * Try memfd first, it's much nicer since it's easily detected thanks to
	 * sealing and also doesn't require assumptions like /tmp.
	 */
	*fdtype = EFD_MEMFD;
	fd = memfd_create(RUNC_MEMFD_COMMENT, MFD_CLOEXEC | MFD_ALLOW_SEALING);
	if (fd >= 0)
		return fd;
	if (errno != ENOSYS)
		goto err;

#ifdef O_TMPFILE
	/*
	 * Try O_TMPFILE to avoid races where someone might snatch our file. Note
	 * that O_EXCL isn't actually a security measure here (since you can just
	 * fd re-open it and clear O_EXCL).
	 */
	*fdtype = EFD_FILE;
	fd = open(prefix, O_TMPFILE | O_EXCL | O_RDWR | O_CLOEXEC, 0700);
	if (fd >= 0) {
		struct stat statbuf = {};
		bool working_otmpfile = false;

		/*
		 * open(2) ignores unknown O_* flags -- yeah, I was surprised when I
		 * found this out too. As a result we can't check for EINVAL. However,
		 * if we get nlink != 0 (or EISDIR) then we know that this kernel
		 * doesn't support O_TMPFILE.
		 */
		if (fstat(fd, &statbuf) >= 0)
			working_otmpfile = (statbuf.st_nlink == 0);

		if (working_otmpfile)
			return fd;

		/* Pretend that we got EISDIR since O_TMPFILE failed. */
		close(fd);
		errno = EISDIR;
	}
	if (errno != EISDIR)
		goto err;
#endif /* defined(O_TMPFILE) */

	/*
	 * Our final option is to create a temporary file the old-school way, and
	 * then unlink it so that nothing else sees it by accident.
	 */
	*fdtype = EFD_FILE;
	fd = mkostemp(template, O_CLOEXEC);
	if (fd >= 0) {
		if (unlink(template) >= 0)
			return fd;
		close(fd);
	}

err:
	*fdtype = EFD_NONE;
	return -1;
}

static int seal_execfd(int *fd, int fdtype)
{
	switch (fdtype) {
	case EFD_MEMFD:
		return fcntl(*fd, F_ADD_SEALS, RUNC_MEMFD_SEALS);
	case EFD_FILE: {
		/* Need to re-open our pseudo-memfd as an O_PATH to avoid execve(2) giving -ETXTBSY. */
		int newfd;
		char fdpath[PATH_MAX] = {0};

		if (fchmod(*fd, 0100) < 0)
			return -1;

		if (snprintf(fdpath, sizeof(fdpath), "/proc/self/fd/%d", *fd) < 0)
			return -1;

		newfd = open(fdpath, O_PATH | O_CLOEXEC);
		if (newfd < 0)
			return -1;

		close(*fd);
		*fd = newfd;
		return 0;
	}
	default:
	   break;
	}
	return -1;
}

static int clone_binary(void)
{
	int binfd, memfd;
	struct stat statbuf = {};
	size_t sent = 0;
	int fdtype = EFD_NONE;

	memfd = make_execfd(&fdtype);
	if (memfd < 0 || fdtype == EFD_NONE)
		return -ENOTRECOVERABLE;

	binfd = open("/proc/self/exe", O_RDONLY | O_CLOEXEC);
	if (binfd < 0)
		goto error;

	if (fstat(binfd, &statbuf) < 0)
		goto error_binfd;

	while (sent < statbuf.st_size) {
		int n = sendfile(memfd, binfd, NULL, statbuf.st_size - sent);
		if (n < 0)
			goto error_binfd;
		sent += n;
	}
	close(binfd);
	if (sent != statbuf.st_size)
		goto error;

	if (seal_execfd(&memfd, fdtype) < 0)
		goto error;
	return memfd;

error_binfd:
	close(binfd);
error:
	close(memfd);
	return -EIO;
}

/* Get cheap access to the environment. */
extern char **environ;

int ensure_cloned_binary(void)
{
	int execfd;
	char **argv = NULL;

	/* Check that we're not self-cloned, and if we are then bail. */
	int cloned = is_self_cloned();
	if (cloned > 0 || cloned == -ENOTRECOVERABLE)
		return cloned;

	if (fetchve(&argv) < 0)
		return -EINVAL;

	execfd = clone_binary();
	if (execfd < 0)
		return -EIO;

	fexecve(execfd, argv, environ);
	return -ENOEXEC;
}
