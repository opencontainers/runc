/*
 * Copyright 2016 SUSE LLC
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
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <unistd.h>

#include "cmsg.h"

#define IB_DATA 'P'

#define error(fmt, ...)							\
	({								\
		fprintf(stderr, "nsenter: " fmt ": %m\n", ##__VA_ARGS__); \
		errno = ECOMM;						\
		return -errno; /* return value */			\
	})

/*
 * Sends a PID to the given @sockfd, which must be an AF_UNIX socket with
 * SO_PASSCRED set. The caller should deal with synchronisation, as this
 * implementation assumes that all of the syscall ordering tomfoolery has been
 * handled. The peer of @sockfd is assumed to be in recvpid() when this is
 * called. The non-ancillary data is set to be some dummy data.
 *
 * In order to send a PID which is not your own, you must have CAP_SYS_ADMIN.
 * In addition, we also just send gete[ug]id().
 */
int sendpid(int sockfd, pid_t pid)
{
	struct msghdr msg = {0};
	struct iovec iov[1] = {{0}};
	struct cmsghdr *cmsg;
	struct ucred *credptr;
	struct ucred cred = {
		.pid = pid,
		.uid = geteuid(),
		.gid = getegid(),
	};
	char ibdata = IB_DATA;

	union {
		char buf[CMSG_SPACE(sizeof(cred))];
		struct cmsghdr align;
	} u;

	/*
	 * We need to send some other data along with the ancillary data,
	 * otherwise the other side won't recieve any data. This is very
	 * well-hidden in the documentation (and only applies to
	 * SOCK_STREAM). See the bottom part of unix(7).
	 */
	iov[0].iov_base = &ibdata;
	iov[0].iov_len = sizeof(ibdata);

	msg.msg_name = NULL;
	msg.msg_namelen = 0;
	msg.msg_iov = iov;
	msg.msg_iovlen = 1;
	msg.msg_control = u.buf;
	msg.msg_controllen = sizeof(u.buf);

	cmsg = CMSG_FIRSTHDR(&msg);
	cmsg->cmsg_level = SOL_SOCKET;
	cmsg->cmsg_type = SCM_CREDENTIALS;
	cmsg->cmsg_len = CMSG_LEN(sizeof(struct ucred));

	credptr = (struct ucred *) CMSG_DATA(cmsg);
	memcpy(credptr, &cred, sizeof(struct ucred));

	return sendmsg(sockfd, &msg, 0);
}

/*
 * Receives a PID from the given @sockfd, which must be an AF_UNIX socket with
 * SO_PASSCRED set. The caller should deal with synchronisation, as this
 * implementation assumes that all of the syscall ordering tomfoolery has been
 * handled. The peer of @sockfd is assumed to be in sendpid() when this is
 * called.
 */
pid_t recvpid(int sockfd)
{
	struct msghdr msg = {0};
	struct iovec iov[1] = {{0}};
	struct cmsghdr *cmsg;
	struct ucred *credptr;
	char ibdata = '\0';
	ssize_t ret;

	union {
		char buf[CMSG_SPACE(sizeof(*credptr))];
		struct cmsghdr align;
	} u;

	/*
	 * We need to "recieve" the non-ancillary data even though we don't
	 * plan to use it at all. Otherwise, things won't work as expected.
	 * See unix(7) and other well-hidden documentation.
	 */
	iov[0].iov_base = &ibdata;
	iov[0].iov_len = sizeof(ibdata);

	msg.msg_name = NULL;
	msg.msg_namelen = 0;
	msg.msg_iov = iov;
	msg.msg_iovlen = 1;
	msg.msg_control = u.buf;
	msg.msg_controllen = sizeof(u.buf);

	ret = recvmsg(sockfd, &msg, 0);
	if (ret < 0)
		return ret;

	cmsg = CMSG_FIRSTHDR(&msg);
	if (!cmsg)
		error("recvfd: got NULL from CMSG_FIRSTHDR");
	if (cmsg->cmsg_level != SOL_SOCKET)
		error("recvfd: expected SOL_SOCKET in cmsg: %d", cmsg->cmsg_level);
	if (cmsg->cmsg_type != SCM_CREDENTIALS)
		error("recvfd: expected SCM_CREDENTIALS in cmsg: %d", cmsg->cmsg_type);
	if (cmsg->cmsg_len != CMSG_LEN(sizeof(struct ucred)))
		error("recvfd: expected correct CMSG_LEN in cmsg: %lu", cmsg->cmsg_len);

	credptr = (struct ucred *) CMSG_DATA(cmsg);
	if (!credptr || !credptr->pid)
		error("recvfd: recieved invalid pointer");

	return credptr->pid;
}
