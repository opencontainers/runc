package nsenter

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

type mockProcessParent struct {
	childPid int
}

func TestNsenterValidPaths(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child := newPipe(t)
	syncParent, syncChild := newPipe(t)

	process, chErr := startMockProcessParent(syncParent)

	namespaces := []string{
		// join pid ns of the current process
		fmt.Sprintf("pid:/proc/%d/ns/pid", os.Getpid()),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child, syncChild},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3", "_LIBCONTAINER_STAGE1PIPE=4"},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("nsenter failed to start: %v", err)
	}
	child.Close()

	// write cloneFlags
	r := nl.NewNetlinkRequest(int(libcontainer.InitMsg), 0)
	r.AddData(&libcontainer.Int32msg{
		Type:  libcontainer.CloneFlagsAttr,
		Value: uint32(unix.CLONE_NEWNET),
	})
	r.AddData(&libcontainer.Bytemsg{
		Type:  libcontainer.NsPathsAttr,
		Value: []byte(strings.Join(namespaces, ",")),
	})
	if _, err := io.Copy(parent, bytes.NewReader(r.Serialize())); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("nsenter error: %v", err)
	}

	if err := <-chErr; err != nil {
		t.Fatal(err)
	}

	reapChildren(t, process)
}

func TestNsenterInvalidPaths(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child := newPipe(t)
	syncParent, syncChild := newPipe(t)

	_, _ = startMockProcessParent(syncParent)

	namespaces := []string{
		fmt.Sprintf("pid:/proc/%d/ns/pid", -1),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child, syncChild},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3", "_LIBCONTAINER_STAGE1PIPE=4"},
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("nsenter failed to start: %v", err)
	}
	child.Close()

	// write cloneFlags
	r := nl.NewNetlinkRequest(int(libcontainer.InitMsg), 0)
	r.AddData(&libcontainer.Int32msg{
		Type:  libcontainer.CloneFlagsAttr,
		Value: uint32(unix.CLONE_NEWNET),
	})
	r.AddData(&libcontainer.Bytemsg{
		Type:  libcontainer.NsPathsAttr,
		Value: []byte(strings.Join(namespaces, ",")),
	})
	if _, err := io.Copy(parent, bytes.NewReader(r.Serialize())); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Wait(); err == nil {
		t.Fatalf("nsenter exits with a zero exit status")
	}
}

func TestNsenterIncorrectPathType(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child := newPipe(t)
	syncParent, syncChild := newPipe(t)

	_, _ = startMockProcessParent(syncParent)

	namespaces := []string{
		fmt.Sprintf("net:/proc/%d/ns/pid", os.Getpid()),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child, syncChild},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3", "_LIBCONTAINER_STAGE1PIPE=4"},
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("nsenter failed to start: %v", err)
	}
	child.Close()

	// write cloneFlags
	r := nl.NewNetlinkRequest(int(libcontainer.InitMsg), 0)
	r.AddData(&libcontainer.Int32msg{
		Type:  libcontainer.CloneFlagsAttr,
		Value: uint32(unix.CLONE_NEWNET),
	})
	r.AddData(&libcontainer.Bytemsg{
		Type:  libcontainer.NsPathsAttr,
		Value: []byte(strings.Join(namespaces, ",")),
	})
	if _, err := io.Copy(parent, bytes.NewReader(r.Serialize())); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Wait(); err == nil {
		t.Fatalf("nsenter error: %v", err)
	}
}

func TestNsenterChildLogging(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child := newPipe(t)
	logread, logwrite := newPipe(t)
	syncParent, syncChild := newPipe(t)

	process, chErr := startMockProcessParent(syncParent)

	namespaces := []string{
		// join pid ns of the current process
		fmt.Sprintf("pid:/proc/%d/ns/pid", os.Getpid()),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child, syncChild, logwrite},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3", "_LIBCONTAINER_STAGE1PIPE=4", "_LIBCONTAINER_LOGPIPE=5"},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("nsenter failed to start: %v", err)
	}
	child.Close()
	logwrite.Close()

	// write cloneFlags
	r := nl.NewNetlinkRequest(int(libcontainer.InitMsg), 0)
	r.AddData(&libcontainer.Int32msg{
		Type:  libcontainer.CloneFlagsAttr,
		Value: uint32(unix.CLONE_NEWNET),
	})
	r.AddData(&libcontainer.Bytemsg{
		Type:  libcontainer.NsPathsAttr,
		Value: []byte(strings.Join(namespaces, ",")),
	})
	if _, err := io.Copy(parent, bytes.NewReader(r.Serialize())); err != nil {
		t.Fatal(err)
	}

	getLogs(t, logread)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("nsenter error: %v", err)
	}

	if err := <-chErr; err != nil {
		t.Fatal(err)
	}

	reapChildren(t, process)
}

func init() {
	if strings.HasPrefix(os.Args[0], "nsenter-") {
		os.Exit(0)
	}
}

func newPipe(t *testing.T) (parent *os.File, child *os.File) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal("socketpair failed:", err)
	}
	parent = os.NewFile(uintptr(fds[1]), "parent")
	child = os.NewFile(uintptr(fds[0]), "child")
	t.Cleanup(func() {
		parent.Close()
		child.Close()
	})
	return
}

func startMockProcessParent(syncSock *os.File) (*mockProcessParent, chan error) {
	process := &mockProcessParent{}
	ch := make(chan error, 1)

	go (func() {
		ch <- libcontainer.ParseNsExecSync(syncSock, func(msg libcontainer.NsExecSyncMsg) error {
			if msg == libcontainer.SyncRecvPidPls {
				var pid uint32
				if err := binary.Read(syncSock, nl.NativeEndian(), &pid); err != nil {
					return err
				}
				process.childPid = int(pid)
				return libcontainer.AckNsExecSync(syncSock, libcontainer.SyncRecvPidAck)
			}
			return nil
		})
	})()

	return process, ch
}

func reapChildren(t *testing.T, parent *mockProcessParent) {
	t.Helper()

	// Sanity check.
	if parent.childPid <= 0 {
		t.Fatal("got pid:", parent.childPid)
	}

	// Reap children.
	_, _ = unix.Wait4(parent.childPid, nil, 0, nil)
}

func getLogs(t *testing.T, logread *os.File) {
	logsDecoder := json.NewDecoder(logread)
	logsDecoder.DisallowUnknownFields()
	var logentry struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	for {
		if err := logsDecoder.Decode(&logentry); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			t.Fatal("init log decoding error:", err)
		}
		t.Logf("logentry: %+v", logentry)
		if logentry.Level == "" || logentry.Msg == "" {
			t.Fatalf("init log: empty log entry: %+v", logentry)
		}
	}
}
