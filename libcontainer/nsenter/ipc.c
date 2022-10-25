#define _GNU_SOURCE
#include <alloca.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include "ipc.h"
#include "log.h"

int receive_fd(int sockfd)
{
	int bytes_read;
	struct msghdr msg = { };
	struct cmsghdr *cmsg;
	struct iovec iov = { };
	char null_byte = '\0';
	int ret;
	int fd_count;

	iov.iov_base = &null_byte;
	iov.iov_len = 1;

	msg.msg_iov = &iov;
	msg.msg_iovlen = 1;

	msg.msg_controllen = CMSG_SPACE(sizeof(int));
	msg.msg_control = malloc(msg.msg_controllen);
	if (msg.msg_control == NULL) {
		bail("Can't allocate memory to receive fd.");
	}

	memset(msg.msg_control, 0, msg.msg_controllen);

	bytes_read = recvmsg(sockfd, &msg, MSG_CMSG_CLOEXEC);
	if (bytes_read != 1)
		bail("failed to receive fd from unix socket %d", sockfd);
	if (msg.msg_flags & MSG_CTRUNC)
		bail("received truncated control message from unix socket %d", sockfd);

	cmsg = CMSG_FIRSTHDR(&msg);
	if (!cmsg)
		bail("received message from unix socket %d without control message", sockfd);

	if (cmsg->cmsg_level != SOL_SOCKET)
		bail("received unknown control message from unix socket %d: cmsg_level=%d", sockfd, cmsg->cmsg_level);

	if (cmsg->cmsg_type != SCM_RIGHTS)
		bail("received unknown control message from unix socket %d: cmsg_type=%d", sockfd, cmsg->cmsg_type);

	fd_count = (cmsg->cmsg_len - CMSG_LEN(0)) / sizeof(int);
	if (fd_count != 1)
		bail("received control message from unix socket %d with too many fds: %d", sockfd, fd_count);

	ret = *(int *)CMSG_DATA(cmsg);
	free(msg.msg_control);
	return ret;
}

int send_fd(int sockfd, int fd)
{
	struct msghdr msg = { };
	struct cmsghdr *cmsg;
	struct iovec iov[1] = { };
	char null_byte = '\0';

	iov[0].iov_base = &null_byte;
	iov[0].iov_len = 1;

	msg.msg_iov = iov;
	msg.msg_iovlen = 1;

	/* We send only one fd as specified by cmsg->cmsg_len below, even
	 * though msg.msg_controllen might have more space due to alignment. */
	msg.msg_controllen = CMSG_SPACE(sizeof(int));
	msg.msg_control = alloca(msg.msg_controllen);
	memset(msg.msg_control, 0, msg.msg_controllen);

	cmsg = CMSG_FIRSTHDR(&msg);
	cmsg->cmsg_level = SOL_SOCKET;
	cmsg->cmsg_type = SCM_RIGHTS;
	cmsg->cmsg_len = CMSG_LEN(sizeof(int));
	memcpy(CMSG_DATA(cmsg), &fd, sizeof(int));

	return sendmsg(sockfd, &msg, 0);
}
