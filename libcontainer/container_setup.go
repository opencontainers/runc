package libcontainer

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/execabs"
	"golang.org/x/sys/unix"
)

// NsExecSyncMsg is used for communication between the parent and child during
// container setup.
type NsExecSyncMsg uint32

const (
	syncUsermapPls NsExecSyncMsg = iota + 0x40
	syncUsermapAck
	syncRecvPidPls
	syncRecvPidAck
	syncTimeOffsetsPls
	syncTimeOffsetsAck
)

type NsExecSetup struct {
	process *containerProcess
}

const bufSize int = 4

// parseNsExecSync runs the given callback function on each message received
// from the child. It will return once the child sends SYNC_RECVPID_PLS.
func parseNsExecSync(r io.Reader, fn func(NsExecSyncMsg) error) error {
	logrus.Debugf("start to communicate with the nsexec\n")
	var msg NsExecSyncMsg
	var buf [bufSize]byte
	native := nl.NativeEndian()

	for {
		if _, err := io.ReadAtLeast(r, buf[:], bufSize); err != nil {
			return err
		}
		msg = NsExecSyncMsg(native.Uint32(buf[:]))
		if err := fn(msg); err != nil {
			return err
		}
		if msg == syncRecvPidPls {
			break
		}
	}
	logrus.Debugf("finished communicating with the nsexec\n")
	return nil
}

// ackSyncMsg is used to send a message to the child.
func ackSyncMsg(f *os.File, msg NsExecSyncMsg) error {
	var buf [bufSize]byte
	native := nl.NativeEndian()
	native.PutUint32(buf[:], uint32(msg))
	if _, err := unix.Write(int(f.Fd()), buf[:]); err != nil {
		logrus.Debugf("failed to write message to nsexec: %v", err)
		return err
	}
	return nil
}

// helpDoingNsExec is used to help the process to communicate with the nsexec.
func (s *NsExecSetup) helpDoingNsExec() error {
	syncSock := s.process.comm.stage1SockParent
	err := parseNsExecSync(syncSock, func(msg NsExecSyncMsg) error {
		switch msg {
		case syncUsermapPls:
			logrus.Debugf("stage-1 requested userns mappings")
			if err := s.setupUsermap(); err != nil {
				return err
			}
			return ackSyncMsg(syncSock, syncUsermapAck)
		case syncRecvPidPls:
			logrus.Debugf("stage-1 reports pid")
			var pid uint32
			if err := binary.Read(syncSock, nl.NativeEndian(), &pid); err != nil {
				return err
			}
			s.process.childPid = int(pid)
			return ackSyncMsg(syncSock, syncRecvPidAck)
		case syncTimeOffsetsPls:
			logrus.Debugf("stage-1 request to configure timens offsets")
			if err := system.UpdateTimeNsOffsets(s.process.cmd.Process.Pid, s.process.container.config.TimeOffsets); err != nil {
				return err
			}
			return ackSyncMsg(syncSock, syncTimeOffsetsAck)
		default:
		}
		return fmt.Errorf("unexpected message %d", msg)
	})
	_ = syncSock.Close()
	return err
}

// setupUsermap is used to set up the user mappings.
func (s *NsExecSetup) setupUsermap() error {
	var uidMapPath, gidMapPath string

	// Enable setgroups(2) if we've been asked to. But we also have to explicitly
	// disable setgroups(2) if we're creating a rootless container for single-entry
	// mapping. (this is required since Linux 3.19).
	// For rootless multi-entry mapping, we should use newuidmap/newgidmap
	// to do mapping user namespace.
	if s.process.config.RootlessEUID && !requiresRootOrMappingTool(s.process.config.Config.GIDMappings) {
		_ = system.UpdateSetgroups(s.process.cmd.Process.Pid, system.SetgroupsDeny)
	}

	nsMaps := make(map[configs.NamespaceType]string)
	for _, ns := range s.process.container.config.Namespaces {
		if ns.Path != "" {
			nsMaps[ns.Type] = ns.Path
		}
	}
	_, joinExistingUser := nsMaps[configs.NEWUSER]
	if !joinExistingUser {
		// write uid mappings
		if len(s.process.container.config.UIDMappings) > 0 {
			if s.process.container.config.RootlessEUID {
				if path, err := execabs.LookPath("newuidmap"); err == nil {
					uidMapPath = path
				}
			}
		}

		// write gid mappings
		if len(s.process.container.config.GIDMappings) > 0 {
			if s.process.container.config.RootlessEUID {
				if path, err := execabs.LookPath("newgidmap"); err == nil {
					gidMapPath = path
				}
			}
		}
	}

	/* Set up mappings. */
	if err := system.UpdateUidmap(uidMapPath, s.process.cmd.Process.Pid, s.process.container.config.UIDMappings); err != nil {
		return err
	}
	return system.UpdateGidmap(gidMapPath, s.process.cmd.Process.Pid, s.process.container.config.GIDMappings)
}
