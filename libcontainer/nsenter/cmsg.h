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

#ifndef NSENTER_CMSG_H
#define NSENTER_CMSG_H

#include <sys/types.h>

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
int sendpid(int sockfd, pid_t pid);

/*
 * Receives a PID from the given @sockfd, which must be an AF_UNIX socket with
 * SO_PASSCRED set. The caller should deal with synchronisation, as this
 * implementation assumes that all of the syscall ordering tomfoolery has been
 * handled. The peer of @sockfd is assumed to be in sendpid() when this is
 * called.
 */
pid_t recvpid(int sockfd);

#endif /* !defined(NSENTER_CMSG_H) */
