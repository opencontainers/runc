#ifndef NSENTER_IPC_H
#define NSENTER_IPC_H

int receive_fd(int sockfd);

/*
 * send_fd passes the open file descriptor fd to another process via the UNIX
 * domain socket sockfd. The return value of the sendmsg(2) call is returned.
 */
int send_fd(int sockfd, int fd);

#endif /* NSENTER_IPC_H */
