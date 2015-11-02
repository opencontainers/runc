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
#define NETLINK_ROUTE		0	
#define NLM_F_ACK             4
#define NLMSG_ERROR            0x2     
#define NLMSG_DATA(nlh)  ((void*)(((char*)nlh) + NLMSG_LENGTH(0)))
#define NLMSG_ALIGNTO  4U
#define NLMSG_ALIGN(len) ( ((len)+NLMSG_ALIGNTO-1) & ~(NLMSG_ALIGNTO-1) )
#define NLMSG_LENGTH(len) ((len) + NLMSG_HDRLEN)
#define NLMSG_HDRLEN    ((int) NLMSG_ALIGN(sizeof(struct nlmsghdr)))
#define TC_H_ROOT    (0xFFFFFFFFU)
#define TCA_KIND    1
#define TIME_UNITS_PER_SEC       1000000
#define NLMSG_TAIL(nmsg) \
	((struct rtattr *) (((void *) (nmsg)) + NLMSG_ALIGN((nmsg)->nlmsg_len)))
#define TCA_BUF_MAX    (64*1024)

struct rtnl_handle
{
    int         fd;                                                                  struct sockaddr_nl  local;
    struct sockaddr_nl  peer;
    __u32           seq;
    __u32           dump;
    int         proto;
#define RTNL_HANDLE_F_LISTEN_ALL_NSID       0x01
    int         flags;
};


struct tc_ratespec {
    unsigned char   cell_log;
    __u8        linklayer; 
    unsigned short  overhead;
    short       cell_align;
    unsigned short  mpu;
    __u32       rate;
};

struct tc_tbf_qopt {
    struct tc_ratespec rate;
    struct tc_ratespec peakrate;
    __u32       limit;
    __u32       buffer;
    __u32       mtu;
};

//tc qdisc add dev eth0 root tbf rate 10mbit burst 10kb latency 70ms  minburst 1540

int add_tc_tbf(int index,int rate_mbit){
	int status;
    struct rtnl_handle rth; 
    __u64 rate_bit=0;
    status=rtnl_open_byproto(&rth, 0, NETLINK_ROUTE);
	if (status < 0) {
		perror("rtnl_open_byproto fail");
		return -1;
	}
    struct {
         struct nlmsghdr     n;
         struct tcmsg        t;
		 char   			buf[TCA_BUF_MAX];
    } req;
    memset(&req, 0, sizeof(req));
    req.t.tcm_family=AF_UNSPEC;
    req.t.tcm_ifindex=index;
    req.t.tcm_parent=TC_H_ROOT;
    req.n.nlmsg_type=36;
    req.n.nlmsg_flags=1537;

	char  k[16];
	memset(&k, 0, sizeof(k));
	strncpy(k, "tbf", sizeof(k)-1);
    req.n.nlmsg_len=36;
	if (k[0])
		addattr_l(&req.n, sizeof(req), TCA_KIND, k, strlen(k)+1);


	struct rtattr *tail;
	tail = NLMSG_TAIL(&req.n);
	struct tc_tbf_qopt opt;
	memset(&opt, 0, sizeof(opt));
    req.n.nlmsg_len=44;
	addattr_l(&req.n, 1024, 2, NULL, 0);
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
	addattr_l(&req.n, 2024, 1, &opt, sizeof(opt));

    req.n.nlmsg_len=88;
	addattr_l(&req.n, 2124,2, &burst, sizeof(burst));
	tail->rta_len = (void *) NLMSG_TAIL(&req.n) - (void *) tail;

	if (rtnl_talk(&rth, &req.n, NULL, 0) < 0)
		return 2;
    return 0;
}

int addattr_l(struct nlmsghdr *n, int maxlen, int type, const void *data,
	      int alen)
{
	int len = RTA_LENGTH(alen);
	struct rtattr *rta;

	if (NLMSG_ALIGN(n->nlmsg_len) + RTA_ALIGN(len) > maxlen) {
        printf("n->nlmsg_len:%d len:%d  maxlen:%d \n",NLMSG_ALIGN(n->nlmsg_len),RTA_ALIGN(len),maxlen);
		fprintf(stderr, "addattr_l ERROR: message exceeded bound of %d\n",maxlen);
		return -1;
	}
	rta = NLMSG_TAIL(n);
	rta->rta_type = type;
	rta->rta_len = len;
	memcpy(RTA_DATA(rta), data, alen);
	n->nlmsg_len = NLMSG_ALIGN(n->nlmsg_len) + RTA_ALIGN(len);
	return 0;
}

int rtnl_talk(struct rtnl_handle *rtnl, struct nlmsghdr *n,
	      struct nlmsghdr *answer, size_t len)
{
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
	nladdr.nl_family = AF_NETLINK;

	n->nlmsg_seq = seq = ++rtnl->seq;

	if (answer == NULL)
		n->nlmsg_flags |= NLM_F_ACK;

	status = sendmsg(rtnl->fd, &msg, 0);
	if (status < 0) {
		perror("Cannot talk to rtnetlink");
		return -1;
	}

	memset(buf,0,sizeof(buf));

	iov.iov_base = buf;
	while (1) {
		iov.iov_len = sizeof(buf);
		status = recvmsg(rtnl->fd, &msg, 0);

		if (status < 0) {
			if (errno == EINTR || errno == EAGAIN)
				continue;
			fprintf(stderr, "netlink receive error %s (%d)\n",
				strerror(errno), errno);
			return -1;
		}
		if (status == 0) {
			fprintf(stderr, "EOF on netlink\n");
			return -1;
		}
		if (msg.msg_namelen != sizeof(nladdr)) {
			fprintf(stderr, "sender address length == %d\n", msg.msg_namelen);
			exit(1);
		}
		for (h = (struct nlmsghdr*)buf; status >= sizeof(*h); ) {
			int len = h->nlmsg_len;
			int l = len - sizeof(*h);

			if (l < 0 || len>status) {
				if (msg.msg_flags & MSG_TRUNC) {
					fprintf(stderr, "Truncated message\n");
					return -1;
				}
				fprintf(stderr, "!!!malformed message: len=%d\n", len);
				exit(1);
			}

			if (nladdr.nl_pid != 0 ||
			    h->nlmsg_pid != rtnl->local.nl_pid ||
			    h->nlmsg_seq != seq) {
				/* Don't forget to skip that message. */
				status -= NLMSG_ALIGN(len);
				h = (struct nlmsghdr*)((char*)h + NLMSG_ALIGN(len));
				continue;
			}

			if (h->nlmsg_type == NLMSG_ERROR) {
				struct nlmsgerr *err = (struct nlmsgerr*)NLMSG_DATA(h);
				if (l < sizeof(struct nlmsgerr)) {
					fprintf(stderr, "ERROR truncated\n");
				} else if (!err->error) {
					if (answer)
						memcpy(answer, h,
						       MIN(len, h->nlmsg_len));
					return 0;
				}

				fprintf(stderr, "RTNETLINK answers: %s\n",
					strerror(-err->error));
				errno = -err->error;
				return -1;
			}

			if (answer) {
				memcpy(answer, h,
				       MIN(len, h->nlmsg_len));
				return 0;
			}

			fprintf(stderr, "Unexpected reply!!!\n");

			status -= NLMSG_ALIGN(len);
			h = (struct nlmsghdr*)((char*)h + NLMSG_ALIGN(len));
		}

		if (msg.msg_flags & MSG_TRUNC) {
			fprintf(stderr, "Message truncated\n");
			continue;
		}

		if (status) {
			fprintf(stderr, "!!!Remnant of size %d\n", status);
			exit(1);
		}
	}
}

int rtnl_open_byproto(struct rtnl_handle *rth, unsigned subscriptions,
		      int protocol)
{
	socklen_t addr_len;
	int sndbuf = 32768;
    int rcvbuf = 1024 * 1024;

	memset(rth, 0, sizeof(*rth));

	rth->proto = protocol;
	rth->fd = socket(AF_NETLINK, SOCK_RAW | SOCK_CLOEXEC, protocol);
	if (rth->fd < 0) {
		perror("Cannot open netlink socket");
		return -1;
	}

	if (setsockopt(rth->fd,SOL_SOCKET,SO_SNDBUF,&sndbuf,sizeof(sndbuf)) < 0) {
		perror("SO_SNDBUF");
		return -1;
	}

	if (setsockopt(rth->fd,SOL_SOCKET,SO_RCVBUF,&rcvbuf,sizeof(rcvbuf)) < 0) {
		perror("SO_RCVBUF");
		return -1;
	}

	memset(&rth->local, 0, sizeof(rth->local));
	rth->local.nl_family = AF_NETLINK;
	rth->local.nl_groups = subscriptions;

	if (bind(rth->fd, (struct sockaddr*)&rth->local, sizeof(rth->local)) < 0) {
		perror("Cannot bind netlink socket");
		return -1;
	}
	addr_len = sizeof(rth->local);
	if (getsockname(rth->fd, (struct sockaddr*)&rth->local, &addr_len) < 0) {
		perror("Cannot getsockname");
		return -1;
	}
	if (addr_len != sizeof(rth->local)) {
		fprintf(stderr, "Wrong address length %d\n", addr_len);
		return -1;
	}
	if (rth->local.nl_family != AF_NETLINK) {
		fprintf(stderr, "Wrong address family %d\n", rth->local.nl_family);
		return -1;
	}
	rth->seq = time(NULL);
	return 0;
}

//struct tcmsg {
//    unsigned char   tcm_family;
//    unsigned char   tcm__pad1;
//    unsigned short  tcm__pad2;
//    int     tcm_ifindex;
//    __u32       tcm_handle;
//    __u32       tcm_parent;
//    __u32       tcm_info;
//};
//struct sockaddr_nl {
//	__kernel_sa_family_t	nl_family;	/* AF_NETLINK	*/
//	unsigned short	nl_pad;		/* zero		*/
//	__u32		nl_pid;		/* port ID	*/
//       	__u32		nl_groups;	/* multicast groups mask */
//};

//struct nlmsghdr {
//    __u32       nlmsg_len;  /* Length of message including header */
//    __u16       nlmsg_type; /* Message content */
//    __u16       nlmsg_flags;    /* Additional flags */                                                                                                                     
//    __u32       nlmsg_seq;  /* Sequence number */
//    __u32       nlmsg_pid;  /* Sending process port ID */
//};
//struct nlmsgerr {
//	int		error;
//	struct nlmsghdr msg;
//};


//int rtnl_open_byproto(struct rtnl_handle *rth, unsigned subscriptions,int protocol) __attribute__((warn_unused_result));
//int rtnl_talk(struct rtnl_handle *rtnl, struct nlmsghdr *n,struct nlmsghdr *answer, size_t len) __attribute__((warn_unused_result));

