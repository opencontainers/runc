package libcontainer

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// NsExecSyncMsg is used for communication between the parent and child during
// container setup.
type NsExecSyncMsg uint32

const (
	SyncUsermapPls NsExecSyncMsg = iota + 0x40
	SyncUsermapAck
	SyncRecvPidPls
	SyncRecvPidAck
	SyncTimeOffsetsPls
	SyncTimeOffsetsAck
)

const bufSize = 4

// setupNsExec is used to help nsexec to setup the container and wait the container's pid.
func (s *containerProcess) setupNsExec(syncSock *os.File) error {
	logrus.Debugf("waiting nsexec to report the container's pid")
	err := ParseNsExecSync(syncSock, func(msg NsExecSyncMsg) error {
		switch msg {
		case SyncUsermapPls:
			logrus.Debugf("nsexec has requested userns mappings")
			if err := s.setupUsermap(); err != nil {
				return err
			}
			return AckNsExecSync(syncSock, SyncUsermapAck)
		case SyncTimeOffsetsPls:
			logrus.Debugf("nsexec has requested to configure timens offsets")
			if err := system.UpdateTimeNsOffsets(s.cmd.Process.Pid, s.container.config.TimeOffsets); err != nil {
				return err
			}
			return AckNsExecSync(syncSock, SyncTimeOffsetsAck)
		case SyncRecvPidPls:
			logrus.Debugf("nsexec has reported pid")
			var pid uint32
			if err := binary.Read(syncSock, nl.NativeEndian(), &pid); err != nil {
				return err
			}
			s.childPid = int(pid)
			return AckNsExecSync(syncSock, SyncRecvPidAck)
		default:
			return fmt.Errorf("unexpected message %d", msg)
		}
	})

	return err
}

// setupUsermap is used to set up the user mappings.
func (s *containerProcess) setupUsermap() error {
	var uidMapPath, gidMapPath string

	// Enable setgroups(2) if we've been asked to. But we also have to explicitly
	// disable setgroups(2) if we're creating a rootless container for single-entry
	// mapping. (this is required since Linux 3.19).
	// For rootless multi-entry mapping, we should use newuidmap/newgidmap
	// to do mapping user namespace.
	if s.config.Config.RootlessEUID && !requiresRootOrMappingTool(s.config.Config.GIDMappings) {
		_ = system.UpdateSetgroups(s.cmd.Process.Pid, system.SetgroupsDeny)
	}

	nsMaps := make(map[configs.NamespaceType]string)
	for _, ns := range s.container.config.Namespaces {
		if ns.Path != "" {
			nsMaps[ns.Type] = ns.Path
		}
	}
	_, joinExistingUser := nsMaps[configs.NEWUSER]
	if !joinExistingUser {
		// write uid mappings
		if len(s.container.config.UIDMappings) > 0 {
			if s.container.config.RootlessEUID {
				if path, err := exec.LookPath("newuidmap"); err == nil {
					uidMapPath = path
				}
			}
		}

		// write gid mappings
		if len(s.container.config.GIDMappings) > 0 {
			if s.container.config.RootlessEUID {
				if path, err := exec.LookPath("newgidmap"); err == nil {
					gidMapPath = path
				}
			}
		}
	}

	/* Set up mappings. */
	if err := system.UpdateUidmap(uidMapPath, s.cmd.Process.Pid, s.container.config.UIDMappings); err != nil {
		return err
	}
	return system.UpdateGidmap(gidMapPath, s.cmd.Process.Pid, s.container.config.GIDMappings)
}

// ParseNsExecSync runs the given callback function on each message received
// from the child. It will return once the child sends SYNC_RECVPID_PLS.
func ParseNsExecSync(r io.Reader, fn func(NsExecSyncMsg) error) error {
	var (
		msg NsExecSyncMsg
		buf [bufSize]byte
	)

	native := nl.NativeEndian()

	for {
		if _, err := io.ReadAtLeast(r, buf[:], bufSize); err != nil {
			return err
		}
		msg = NsExecSyncMsg(native.Uint32(buf[:]))
		if err := fn(msg); err != nil {
			return err
		}
		if msg == SyncRecvPidPls {
			break
		}
	}
	return nil
}

// AckNsExecSync is used to send a message to the child.
func AckNsExecSync(f *os.File, msg NsExecSyncMsg) error {
	var buf [bufSize]byte
	native := nl.NativeEndian()
	native.PutUint32(buf[:], uint32(msg))
	if _, err := unix.Write(int(f.Fd()), buf[:]); err != nil {
		logrus.Debugf("failed to write message to nsexec: %v", err)
		return err
	}
	return nil
}
