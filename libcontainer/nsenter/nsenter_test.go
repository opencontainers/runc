package nsenter

import (
	"bytes"
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

func TestNsenterValidPaths(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child := newPipe(t)

	namespaces := []string{
		// join pid ns of the current process
		fmt.Sprintf("pid:/proc/%d/ns/pid", os.Getpid()),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3"},
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

	initWaiter(t, parent)

	if err := cmd.Wait(); err != nil {
		t.Fatalf("nsenter error: %v", err)
	}

	reapChildren(t, parent)
}

func TestNsenterInvalidPaths(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child := newPipe(t)

	namespaces := []string{
		fmt.Sprintf("pid:/proc/%d/ns/pid", -1),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3"},
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

	initWaiter(t, parent)
	if err := cmd.Wait(); err == nil {
		t.Fatalf("nsenter exits with a zero exit status")
	}
}

func TestNsenterIncorrectPathType(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child := newPipe(t)

	namespaces := []string{
		fmt.Sprintf("net:/proc/%d/ns/pid", os.Getpid()),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3"},
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

	initWaiter(t, parent)
	if err := cmd.Wait(); err == nil {
		t.Fatalf("nsenter error: %v", err)
	}
}

func TestNsenterChildLogging(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child := newPipe(t)
	logread, logwrite := newPipe(t)

	namespaces := []string{
		// join pid ns of the current process
		fmt.Sprintf("pid:/proc/%d/ns/pid", os.Getpid()),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child, logwrite},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3", "_LIBCONTAINER_LOGPIPE=4"},
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

	initWaiter(t, parent)

	getLogs(t, logread)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("nsenter error: %v", err)
	}

	reapChildren(t, parent)
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

// initWaiter reads back the initial \0 from runc init
func initWaiter(t *testing.T, r io.Reader) {
	inited := make([]byte, 1)
	n, err := r.Read(inited)
	if err == nil {
		if n < 1 {
			err = errors.New("short read")
		} else if inited[0] != 0 {
			err = fmt.Errorf("unexpected %d != 0", inited[0])
		} else {
			return
		}
	}
	t.Fatalf("waiting for init preliminary setup: %v", err)
}

func reapChildren(t *testing.T, parent *os.File) {
	t.Helper()
	decoder := json.NewDecoder(parent)
	decoder.DisallowUnknownFields()
	var pid struct {
		Pid2 int `json:"stage2_pid"`
		Pid1 int `json:"stage1_pid"`
	}
	if err := decoder.Decode(&pid); err != nil {
		t.Fatal(err)
	}

	// Reap children.
	_, _ = unix.Wait4(pid.Pid1, nil, 0, nil)
	_, _ = unix.Wait4(pid.Pid2, nil, 0, nil)

	// Sanity check.
	if pid.Pid1 == 0 || pid.Pid2 == 0 {
		t.Fatal("got pids:", pid)
	}
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
