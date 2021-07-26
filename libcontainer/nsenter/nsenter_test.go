package nsenter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
	parent, child, err := newPipe()
	if err != nil {
		t.Fatalf("failed to create pipe %v", err)
	}

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
		t.Fatalf("nsenter failed to start %v", err)
	}

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

	decoder := json.NewDecoder(parent)
	var pid struct {
		Pid int
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("nsenter error: %v", err)
	}
	if err := decoder.Decode(&pid); err != nil {
		dir, _ := ioutil.ReadDir(fmt.Sprintf("/proc/%d/ns", os.Getpid()))
		for _, d := range dir {
			t.Log(d.Name())
		}
		t.Fatalf("%v", err)
	}
	t.Logf("got pid: %d", pid.Pid)

	p, err := os.FindProcess(pid.Pid)
	if err != nil {
		t.Fatalf("%v", err)
	}
	_, _ = p.Wait()
}

func TestNsenterInvalidPaths(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child, err := newPipe()
	if err != nil {
		t.Fatalf("failed to create pipe %v", err)
	}

	namespaces := []string{
		// join pid ns of the current process
		fmt.Sprintf("pid:/proc/%d/ns/pid", -1),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3"},
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
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

func TestNsenterIncorrectPathType(t *testing.T) {
	args := []string{"nsenter-exec"}
	parent, child, err := newPipe()
	if err != nil {
		t.Fatalf("failed to create pipe %v", err)
	}

	namespaces := []string{
		// join pid ns of the current process
		fmt.Sprintf("net:/proc/%d/ns/pid", os.Getpid()),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child},
		Env:        []string{"_LIBCONTAINER_INITPIPE=3"},
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
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
	parent, child, err := newPipe()
	if err != nil {
		t.Fatalf("failed to create exec pipe %v", err)
	}
	logread, logwrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create log pipe %v", err)
	}
	defer func() {
		_ = logwrite.Close()
		_ = logread.Close()
	}()

	namespaces := []string{
		// join pid ns of the current process
		fmt.Sprintf("pid:/proc/%d/ns/pid", os.Getpid()),
	}
	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{child, logwrite},
		Env: []string{
			"_LIBCONTAINER_INITPIPE=3",
			"_LIBCONTAINER_LOGPIPE=4",
			"_LIBCONTAINER_LOGLEVEL=5", // DEBUG
		},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("nsenter failed to start %v", err)
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

	logsDecoder := json.NewDecoder(logread)
	var logentry struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	for {
		err = logsDecoder.Decode(&logentry)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("child log: %v", err)
		}
		t.Logf("logentry: %+v", logentry)
		if logentry.Level == "" || logentry.Msg == "" {
			t.Fatalf("child log: empty log entry: %+v", logentry)
		}
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("nsenter error: %v", err)
	}
}

func init() {
	if strings.HasPrefix(os.Args[0], "nsenter-") {
		os.Exit(0)
	}
}

func newPipe() (parent *os.File, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
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
