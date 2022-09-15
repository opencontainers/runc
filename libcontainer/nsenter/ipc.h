#ifndef NSENTER_IPC_H
#define NSENTER_IPC_H

void receive_fd(int sockfd, int new_fd);
void send_fd(int sockfd, int fd);

#endif /* NSENTER_IPC_H */
