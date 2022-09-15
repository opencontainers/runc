#define _GNU_SOURCE
#include <fcntl.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <unistd.h>
#include "ipc.h"
#include "log.h"

void receive_fd(int sockfd, int new_fd)
{
	int bytes_read;
	struct msghdr msg = { };
	struct cmsghdr *cmsg;
	struct iovec iov = { };
	char null_byte = '\0';
	int ret;
	int fd_count;
	int *fd_payload;

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

	bytes_read = recvmsg(sockfd, &msg, 0);
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

	fd_payload = (int *)CMSG_DATA(cmsg);
	ret = dup3(*fd_payload, new_fd, O_CLOEXEC);
	if (ret < 0)
		bail("cannot dup3 fd %d to %d", *fd_payload, new_fd);

	free(msg.msg_control);

	ret = close(*fd_payload);
	if (ret < 0)
		bail("cannot close fd %d", *fd_payload);
}

void send_fd(int sockfd, int fd)
{
	int bytes_written;
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
	msg.msg_control = malloc(msg.msg_controllen);
	if (msg.msg_control == NULL) {
		bail("Can't allocate memory to send fd.");
	}

	memset(msg.msg_control, 0, msg.msg_controllen);

	cmsg = CMSG_FIRSTHDR(&msg);
	cmsg->cmsg_level = SOL_SOCKET;
	cmsg->cmsg_type = SCM_RIGHTS;
	cmsg->cmsg_len = CMSG_LEN(sizeof(int));
	memcpy(CMSG_DATA(cmsg), &fd, sizeof(int));

	bytes_written = sendmsg(sockfd, &msg, 0);

	free(msg.msg_control);

	if (bytes_written != 1)
		bail("failed to send fd %d via unix socket %d", fd, sockfd);
}
