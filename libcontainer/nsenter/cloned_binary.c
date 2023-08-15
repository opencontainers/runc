// SPDX-License-Identifier: Apache-2.0 OR LGPL-2.1-or-later
/*
 * Copyright (C) 2019 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2019 SUSE LLC
 *
 * This work is dual licensed under the following licenses. You may use,
 * redistribute, and/or modify the work under the conditions of either (or
 * both) licenses.
 *
 * === Apache-2.0 ===
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
 *
 * === LGPL-2.1-or-later ===
 *
 * This library is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * This library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public
 * License along with this library. If not, see
 * <https://www.gnu.org/licenses/>.
 *
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

#include <sched.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/statfs.h>
#include <sys/vfs.h>
#include <sys/mman.h>
#include <sys/mount.h>
#include <sys/sendfile.h>
#include <sys/socket.h>
#include <sys/syscall.h>
#include <sys/wait.h>

#include "ipc.h"
#include "log.h"

/* Use our own wrapper for memfd_create. */
#ifndef SYS_memfd_create
#  ifdef __NR_memfd_create
#    define SYS_memfd_create __NR_memfd_create
#  else
/* These values come from <https://fedora.juszkiewicz.com.pl/syscalls.html>. */
#    warning "libc is outdated -- using hard-coded SYS_memfd_create"
#    if defined(__x86_64__)
#      define SYS_memfd_create 319
#    elif defined(__i386__)
#      define SYS_memfd_create 356
#    elif defined(__ia64__)
#      define SYS_memfd_create 1340
#    elif defined(__arm__)
#      define SYS_memfd_create 385
#    elif defined(__aarch64__)
#      define SYS_memfd_create 279
#    elif defined(__ppc__) || defined(__PPC64__) || defined(__powerpc64__)
#      define SYS_memfd_create 360
#    elif defined(__s390__) || defined(__s390x__)
#      define SYS_memfd_create 350
#    else
#      warning "unknown architecture -- cannot hard-code SYS_memfd_create"
#    endif
#  endif
#endif

/* memfd_create(2) flags -- copied from <linux/memfd.h>. */
#ifndef MFD_CLOEXEC
#  define MFD_CLOEXEC       0x0001U
#  define MFD_ALLOW_SEALING 0x0002U
#endif
#ifndef MFD_EXEC
#  define MFD_EXEC          0x0010U
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
#  define F_SEAL_SEAL          0x0001	/* prevent further seals from being set */
#  define F_SEAL_SHRINK        0x0002	/* prevent file from shrinking */
#  define F_SEAL_GROW          0x0004	/* prevent file from growing */
#  define F_SEAL_WRITE         0x0008	/* prevent writes */
#endif
#ifndef F_SEAL_FUTURE_WRITE
#  define F_SEAL_FUTURE_WRITE  0x0010	/* prevent future writes while mapped */
#endif
#ifndef F_SEAL_EXEC
#  define F_SEAL_EXEC          0x0020	/* prevent chmod modifying exec bits */
#endif

#define CLONED_BINARY_ENV "_LIBCONTAINER_CLONED_BINARY"
#define RUNC_MEMFD_COMMENT "runc_cloned:runc-dmz"
/*
 * There are newer memfd seals (such as F_SEAL_FUTURE_WRITE and F_SEAL_EXEC),
 * which we use opportunistically. However, this set is the original set of
 * memfd seals, and we require them all to be set to trust our /proc/self/exe
 * if it is a memfd.
 */
#define RUNC_MEMFD_MIN_SEALS \
	(F_SEAL_SEAL | F_SEAL_SHRINK | F_SEAL_GROW | F_SEAL_WRITE)

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

static inline bool is_memfd_unsupported_error(int err)
{
	/*
	 * - ENOSYS is obviously an "unsupported" error.
	 *
	 * - EINVAL could be hit if MFD_EXEC is not supported (pre-6.3 kernel),
	 *   but it can also be hit if vm.memfd_noexec=2 (in kernels without
	 *   [1] applied) and the flags does not contain MFD_EXEC. However,
	 *   there was a bug in the original 6.3 implementation of
	 *   vm.memfd_noexec=2, which meant that MFD_EXEC would work even in
	 *   the "strict" mode. Because we try MFD_EXEC first, we won't get
	 *   EINVAL in the vm.memfd_noexec=2 case (which means we don't need to
	 *   figure out whether to log the message about memfd_create).
	 *
	 * - EACCES is returned in kernels that contain [1] in the
	 *   vm.memfd_noexec=2 case.
	 *
	 * At time of writing, [1] is not in Linus's tree and it't not clear if
	 * it will be backported to stable, so what exact versions apply here
	 * is unclear. But the bug is present in 6.3-6.5 at the very least.
	 *
	 * [1]: https://lore.kernel.org/all/20230705063315.3680666-2-jeffxu@google.com/
	 */
	if (err == EACCES)
		write_log(INFO,
			  "memfd_create(MFD_EXEC) failed, possibly due to vm.memfd_noexec=2 -- falling back to less secure O_TMPFILE");
	return err == ENOSYS || err == EINVAL || err == EACCES;
}

static int make_execfd(int *fdtype)
{
	int fd = -1;
	char template[PATH_MAX] = { 0 };
	char *prefix = getenv("_LIBCONTAINER_STATEDIR");

	if (!prefix || *prefix != '/')
		prefix = "/tmp";
	if (snprintf(template, sizeof(template), "%s/runc.XXXXXX", prefix) < 0)
		return -1;

	/*
	 * Now try memfd, it's much nicer than actually creating a file in STATEDIR
	 * since it's easily detected thanks to sealing and also doesn't require
	 * assumptions about STATEDIR.
	 */
	*fdtype = EFD_MEMFD;
	/*
	 * On newer kernels we should set MFD_EXEC to indicate we need +x
	 * permissions. Otherwise an admin with vm.memfd_noexec=1 would subtly
	 * break runc. vm.memfd_noexec=2 is a little bit more complicated, see the
	 * comment in is_memfd_unsupported_error() -- the upshot is that doing it
	 * this way works, but only because of two overlapping bugs in the sysctl
	 * implementation.
	 */
	fd = memfd_create(RUNC_MEMFD_COMMENT, MFD_EXEC | MFD_CLOEXEC | MFD_ALLOW_SEALING);
	if (fd < 0 && is_memfd_unsupported_error(errno))
		fd = memfd_create(RUNC_MEMFD_COMMENT, MFD_CLOEXEC | MFD_ALLOW_SEALING);
	if (fd >= 0)
		return fd;
	if (!is_memfd_unsupported_error(errno))
		goto error;

#ifdef O_TMPFILE
	/*
	 * Try O_TMPFILE to avoid races where someone might snatch our file. Note
	 * that O_EXCL isn't actually a security measure here (since you can just
	 * fd re-open it and clear O_EXCL).
	 */
	*fdtype = EFD_FILE;
	fd = open(prefix, O_TMPFILE | O_EXCL | O_RDWR | O_CLOEXEC, 0700);
	if (fd >= 0) {
		struct stat statbuf = { };
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
		goto error;
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

error:
	*fdtype = EFD_NONE;
	return -1;
}

static int seal_execfd(int *fd, int fdtype)
{
	switch (fdtype) {
	case EFD_MEMFD:{
			/*
			 * Try to seal with newer seals, but we ignore errors because older
			 * kernels don't support some of them. For container security only
			 * RUNC_MEMFD_MIN_SEALS are strictly required, but the rest are
			 * nice-to-haves. We apply RUNC_MEMFD_MIN_SEALS at the end because it
			 * contains F_SEAL_SEAL.
			 */
			int __attribute__((unused)) _err1 = fcntl(*fd, F_ADD_SEALS, F_SEAL_FUTURE_WRITE);	// Linux 5.1
			int __attribute__((unused)) _err2 = fcntl(*fd, F_ADD_SEALS, F_SEAL_EXEC);	// Linux 6.3
			return fcntl(*fd, F_ADD_SEALS, RUNC_MEMFD_MIN_SEALS);
		}
	case EFD_FILE:{
			/* Need to re-open our pseudo-memfd as an O_PATH to avoid execve(2) giving -ETXTBSY. */
			int newfd;
			char fdpath[PATH_MAX] = { 0 };

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

static ssize_t fd_to_fd(int outfd, int infd)
{
	ssize_t total = 0;
	char buffer[4096];

	for (;;) {
		ssize_t nread, nwritten = 0;

		nread = read(infd, buffer, sizeof(buffer));
		if (nread < 0)
			return -1;
		if (!nread)
			break;

		do {
			ssize_t n = write(outfd, buffer + nwritten, nread - nwritten);
			if (n < 0)
				return -1;
			nwritten += n;
		} while (nwritten < nread);

		total += nwritten;
	}

	return total;
}

static int clone_binary(void)
{
	int binfd, execfd;
	struct stat statbuf = { };
	size_t sent = 0;
	int fdtype = EFD_NONE;
	char runcpath[PATH_MAX] = { 0 };
	char dmzpath[PATH_MAX] = { 0 };

	execfd = make_execfd(&fdtype);
	if (execfd < 0 || fdtype == EFD_NONE)
		return -ENOTRECOVERABLE;

	if (readlink("/proc/self/exe", runcpath, PATH_MAX) < 1)
		goto error;

	if (snprintf(dmzpath, PATH_MAX, "%s%s", runcpath, "-dmz") < 0)
		goto error;

	binfd = open(dmzpath, O_RDONLY | O_CLOEXEC);
	if (binfd < 0)
		goto error;

	if (fstat(binfd, &statbuf) < 0)
		goto error_binfd;

	while (sent < statbuf.st_size) {
		int n = sendfile(execfd, binfd, NULL, statbuf.st_size - sent);
		if (n < 0) {
			/* sendfile can fail so we fallback to a dumb user-space copy. */
			n = fd_to_fd(execfd, binfd);
			if (n < 0)
				goto error_binfd;
		}
		sent += n;
	}
	close(binfd);
	if (sent != statbuf.st_size)
		goto error;

	if (seal_execfd(&execfd, fdtype) < 0)
		goto error;

	return execfd;

error_binfd:
	close(binfd);
error:
	close(execfd);
	return -EIO;
}

/* Get cheap access to the environment. */
extern char **environ;

int ensure_cloned_binary(void)
{
	int execfd;

	execfd = clone_binary();
	if (execfd < 0)
		return -EIO;

	char envString[PATH_MAX] = { 0 };
	if (sprintf(envString, "%d", execfd) < 0)
		goto error;

	if (setenv("_LIBCONTAINER_DMZFD", envString, 1))
		goto error;

	return 0;
error:
	close(execfd);
	return -ENOEXEC;
}
