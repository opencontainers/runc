package images

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
)

func ProtoHandler(magic string) (proto.Message, error) {
	switch magic {
	case "APPARMOR":
		return &ApparmorEntry{}, nil
	case "AUTOFS":
		return &AutofsEntry{}, nil
	case "BINFMT_MISC":
		return &BinfmtMiscEntry{}, nil
	case "BPFMAP_DATA":
		return &BpfmapDataEntry{}, nil
	case "BPFMAP_FILE":
		return &BpfmapFileEntry{}, nil
	case "CGROUP":
		return &CgroupEntry{}, nil
	case "CORE":
		return &CoreEntry{}, nil
	case "CPUINFO":
		return &CpuinfoEntry{}, nil
	case "CREDS":
		return &CredsEntry{}, nil
	case "EVENTFD_FILE":
		return &EventfdFileEntry{}, nil
	case "EVENTPOLL_FILE":
		return &EventpollFileEntry{}, nil
	case "EVENTPOLL_TFD":
		return &EventpollTfdEntry{}, nil
	case "EXT_FILES":
		return &ExtFileEntry{}, nil
	case "FANOTIFY_FILE":
		return &FanotifyFileEntry{}, nil
	case "FANOTIFY_MARK":
		return &FanotifyMarkEntry{}, nil
	case "FDINFO":
		return &FdinfoEntry{}, nil
	case "FIFO":
		return &FifoEntry{}, nil
	case "FIFO_DATA":
		return &PipeDataEntry{}, nil
	case "FILES":
		return &FileEntry{}, nil
	case "FILE_LOCKS":
		return &FileLockEntry{}, nil
	case "FS":
		return &FsEntry{}, nil
	case "IDS":
		return &TaskKobjIdsEntry{}, nil
	case "INETSK":
		return &InetSkEntry{}, nil
	case "INOTIFY_FILE":
		return &InotifyFileEntry{}, nil
	case "INOTIFY_WD":
		return &InotifyWdEntry{}, nil
	case "INVENTORY":
		return &InventoryEntry{}, nil
	case "IPCNS_MSG":
		return &IpcMsgEntry{}, nil
	case "IPCNS_SEM":
		return &IpcSemEntry{}, nil
	case "IPCNS_SHM":
		return &IpcShmEntry{}, nil
	case "IPC_VAR":
		return &IpcVarEntry{}, nil
	case "IRMAP_CACHE":
		return &IrmapCacheEntry{}, nil
	case "ITIMERS":
		return &ItimerEntry{}, nil
	case "MEMFD_INODE":
		return &MemfdInodeEntry{}, nil
	case "MM":
		return &MmEntry{}, nil
	case "MNTS":
		return &MntEntry{}, nil
	case "NETDEV":
		return &NetDeviceEntry{}, nil
	case "NETLINK_SK":
		return &NetlinkSkEntry{}, nil
	case "NETNS":
		return &NetnsEntry{}, nil
	case "NS_FILES":
		return &NsFileEntry{}, nil
	case "PACKETSK":
		return &PacketSockEntry{}, nil
	case "PIDNS":
		return &PidnsEntry{}, nil
	case "PIPES":
		return &PipeEntry{}, nil
	case "PIPES_DATA":
		return &PipeDataEntry{}, nil
	case "POSIX_TIMERS":
		return &PosixTimerEntry{}, nil
	case "PSTREE":
		return &PstreeEntry{}, nil
	case "REG_FILES":
		return &RegFileEntry{}, nil
	case "REMAP_FPATH":
		return &RemapFilePathEntry{}, nil
	case "RLIMIT":
		return &RlimitEntry{}, nil
	case "SECCOMP":
		return &SeccompEntry{}, nil
	case "SIGACT":
		return &SaEntry{}, nil
	case "SIGNALFD":
		return &SignalfdEntry{}, nil
	case "SK_QUEUES":
		return &SkPacketEntry{}, nil
	case "STATS":
		return &StatsEntry{}, nil
	case "TCP_STREAM":
		return &TcpStreamEntry{}, nil
	case "TIMENS":
		return &TimensEntry{}, nil
	case "TIMERFD":
		return &TimerfdEntry{}, nil
	case "TTY_DATA":
		return &TtyDataEntry{}, nil
	case "TTY_FILES":
		return &TtyFileEntry{}, nil
	case "TTY_INFO":
		return &TtyInfoEntry{}, nil
	case "TUNFILE":
		return &TunfileEntry{}, nil
	case "UNIXSK":
		return &UnixSkEntry{}, nil
	case "USERNS":
		return &UsernsEntry{}, nil
	case "UTSNS":
		return &UtsnsEntry{}, nil
	case "VMAS":
		return &VmaEntry{}, nil
	}
	return nil, errors.New(fmt.Sprintf("No handler found for magic 0x%x", magic))
}
