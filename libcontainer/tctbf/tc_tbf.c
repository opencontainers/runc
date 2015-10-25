#include <sys/types.h>
#include <stdio.h>
#include <sys/socket.h>
#include <linux/socket.h> 
#include <linux/kernel.h>
#include <string.h>
#include <errno.h>
#include <stdlib.h>
#include <unistd.h>
#include <asm/types.h>
#include <linux/netlink.h>
#include <linux/rtnetlink.h>
#include <sys/socket.h>

#define MIN(a, b) ((a) < (b) ? (a) : (b))
#define NL_MSG_DATA(nlh)  ((void*)(((char*)nlh) + NL_MSG_LENGTH(0)))
#define NL_MSG_ALIGNTO  4U
#define NL_MSG_ALIGN(len) ( ((len)+NL_MSG_ALIGNTO-1) & ~(NL_MSG_ALIGNTO-1) )
#define NL_MSG_LENGTH(len) ((len) + NL_MSG_HDRLEN)
#define NL_MSG_HDRLEN    ((int) NL_MSG_ALIGN(sizeof(struct nlmsghdr)))
#define NL_MSG_TAIL(nmsg) \
	((struct rtattr *) (((void *) (nmsg)) + NL_MSG_ALIGN((nmsg)->nlmsg_len)))


struct tc_rate_desc {
    unsigned char   cell_log;
    __u8        linklayer; 
    unsigned short  overhead;
    short       cell_align;
    unsigned short  mpu;
    __u32       rate;
};

struct tc_tbf_opt {
    struct tc_rate_desc rate;
    struct tc_rate_desc peakrate;
    __u32       limit;
    __u32       buffer;
    __u32       mtu;
};

void addrta(struct nlmsghdr *n, int type, const void *data,int alen);

//tc qdisc add dev eth0 root tbf rate 10mbit burst 10kb latency 70ms  minburst 1540
int add_tc_tbf(int index,int rate_mbit){
    __u64 rate_bit=0;

    struct {
         struct nlmsghdr     n;
         struct tcmsg        t;
		 char   			buf[8*1024];
    } req;
    memset(&req, 0, sizeof(req));
    req.t.tcm_family=AF_UNSPEC;
    req.t.tcm_ifindex=index;
    req.t.tcm_parent=0xFFFFFFFFU;
    req.n.nlmsg_type=36;
    req.n.nlmsg_flags=1537;

	char  k[16];
	memset(&k, 0, sizeof(k));
	strncpy(k, "tbf", sizeof(k)-1);
    req.n.nlmsg_len=36;
	addrta(&req.n,1, k, strlen(k)+1);

	struct rtattr *tail;
	tail = NL_MSG_TAIL(&req.n);
	struct tc_tbf_opt opt;
	memset(&opt, 0, sizeof(opt));
    req.n.nlmsg_len=44;
	addrta(&req.n, 2, NULL, 0);
    opt.rate.cell_log=3;
    opt.rate.linklayer=1;
    opt.rate.cell_align=-1;
    opt.rate.mpu=0;
    rate_bit=rate_mbit*125000;
    opt.rate.rate=rate_bit;
    // burst 
    unsigned burst=rate_bit/1000;
    //  latency = 70 ,burst= ratebit/1000
    opt.limit=70*rate_bit+burst;
    opt.buffer=rate_bit/burst * 128;
    req.n.nlmsg_len=48;
	addrta(&req.n, 1, &opt, sizeof(opt));

    req.n.nlmsg_len=88;
	addrta(&req.n, 2124, &burst, sizeof(burst));
	tail->rta_len = (void *) NL_MSG_TAIL(&req.n) - (void *) tail;
    return nl_sendmsg(&req.n);
}

void addrta(struct nlmsghdr *n, int type, const void *data,int alen)
{
	int len = RTA_LENGTH(alen);
	struct rtattr *rta;
	rta = NL_MSG_TAIL(n);
	rta->rta_type = type;
	rta->rta_len = len;
	memcpy(RTA_DATA(rta), data, alen);
	n->nlmsg_len = NL_MSG_ALIGN(n->nlmsg_len) + RTA_ALIGN(len);
}

int nl_sendmsg(struct nlmsghdr *n)
{
	socklen_t sl;
    struct sockaddr_nl snl;
    int rbuf = 1024 * 1024;
	int sbuf = 32768;
    int fd;
    struct nlmsghdr *answer;
	int status;
	unsigned seq;
	struct nlmsghdr *h;
	struct sockaddr_nl nladdr;
	struct iovec iov = {
		.iov_base = (void*) n,
		.iov_len = n->nlmsg_len
	};
	struct msghdr msg = {
		.msg_name = &nladdr,
		.msg_namelen = sizeof(nladdr),
		.msg_iov = &iov,
		.msg_iovlen = 1,
	};
	char   buf[32768];
	memset(&nladdr, 0, sizeof(nladdr));

//set_fd
	fd = socket(AF_NETLINK, SOCK_RAW | SOCK_CLOEXEC, 0);
	if (fd < 0) {
		perror("Cannot open netlink socket");
		return -1;
	}

	if (setsockopt(fd,SOL_SOCKET,SO_SNDBUF,&sbuf,sizeof(sbuf)) < 0) {
		perror("Send bufer set fail");
		return -2;
	}

	if (setsockopt(fd,SOL_SOCKET,SO_RCVBUF,&rbuf,sizeof(rbuf)) < 0) {
		perror("Receive bufer set fail");
		return -3;
	}

	memset(&snl, 0, sizeof(snl));
	snl.nl_family = AF_NETLINK;
	snl.nl_groups = 0;

	if (bind(fd, (struct sockaddr*)&snl, sizeof(snl)) < 0) {
		perror("Cannot bind netlink socket");
		return -4;
	}
	sl = sizeof(snl);
	if (getsockname(fd, (struct sockaddr*)&snl, &sl) < 0) {
		perror("getsockname Fail");
		return -5;
	}
	if (sl != sizeof(snl)) {
		fprintf(stderr, "Wrong address length %d\n", sl);
		return -6;
	}
	if (snl.nl_family != AF_NETLINK) {
		fprintf(stderr, "Wrong address family %d\n", snl.nl_family);
		return -7;
	}

//sendmsg

	nladdr.nl_family = AF_NETLINK;
    seq = time(NULL);
	n->nlmsg_seq = ++seq;

	n->nlmsg_flags |= 4;

	status = sendmsg(fd, &msg, 0);
	if (status < 0) {
		perror("Cannot talk to rtnetlink");
		return -8;
	}

	memset(buf,0,sizeof(buf));

	iov.iov_base = buf;
	while (1) {
		iov.iov_len = sizeof(buf);
		status = recvmsg(fd, &msg, 0);

		if (status < 0) {
			if (errno == EINTR || errno == EAGAIN)
				continue;
			fprintf(stderr, "netlink receive error %s (%d)\n",
				strerror(errno), errno);
            close(fd);
			return -9;
		}
		if (status == 0) {
			fprintf(stderr, "EOF on netlink\n");
			return -10;
		}
		if (msg.msg_namelen != sizeof(nladdr)) {
			fprintf(stderr, "sender address length == %d\n", msg.msg_namelen);
            close(fd);
            return -11;
		}
		for (h = (struct nlmsghdr*)buf; status >= sizeof(*h); ) {
			int len = h->nlmsg_len;
			int l = len - sizeof(*h);

			if (l < 0 || len>status) {
				if (msg.msg_flags & MSG_TRUNC) {
					fprintf(stderr, "Truncated message\n");
                    close(fd);
					return -12;
				}
				fprintf(stderr, "!!!malformed message: len=%d\n", len);
                close(fd);
                return -13;
			}

			if (nladdr.nl_pid != 0 ||
			    h->nlmsg_pid != snl.nl_pid ||
			    h->nlmsg_seq != seq) {
				status -= NL_MSG_ALIGN(len);
				h = (struct nlmsghdr*)((char*)h + NL_MSG_ALIGN(len));
				continue;
			}

			if (h->nlmsg_type == 0x2) {
				struct nlmsgerr *err = (struct nlmsgerr*)NL_MSG_DATA(h);
				if (l < sizeof(struct nlmsgerr)) {
					fprintf(stderr, "ERROR truncated\n");
				} else if (!err->error) {
					if (answer)
						memcpy(answer, h,
						       MIN(len, h->nlmsg_len));
                
                    close(fd);
					return 0;
				}

				fprintf(stderr, "RTNETLINK answers: %s\n",
					strerror(-err->error));
				errno = -err->error;
                close(fd);
				return -14;
			}

			if (answer) {
				memcpy(answer, h,
				       MIN(len, h->nlmsg_len));

                close(fd);
				return 0;
			}

			fprintf(stderr, "Unexpected reply!!!\n");

			status -= NL_MSG_ALIGN(len);
			h = (struct nlmsghdr*)((char*)h + NL_MSG_ALIGN(len));
		}

		if (msg.msg_flags & MSG_TRUNC) {
			fprintf(stderr, "Message truncated\n");
			continue;
		}

		if (status) {
			fprintf(stderr, "!!!Remnant of size %d\n", status);
            close(fd);
            return -15;
		}
	}
}

