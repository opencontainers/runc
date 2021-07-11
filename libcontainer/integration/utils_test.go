package integration

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
)

var busyboxTar string

// init makes sure the container images are downloaded,
// and initializes busyboxTar. If images can't be downloaded,
// we are unable to run any tests, so panic.
func init() {
	// Figure out path to get-images.sh. Note it won't work
	// in case the compiled test binary is moved elsewhere.
	_, ex, _, _ := runtime.Caller(0)
	getImages, err := filepath.Abs(filepath.Join(filepath.Dir(ex), "..", "..", "tests", "integration", "get-images.sh"))
	if err != nil {
		panic(err)
	}
	// Call it to make sure images are downloaded, and to get the paths.
	out, err := exec.Command(getImages).CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("getImages error %s (output: %s)", err, out))
	}
	// Extract the value of BUSYBOX_IMAGE.
	found := regexp.MustCompile(`(?m)^BUSYBOX_IMAGE=(.*)$`).FindSubmatchIndex(out)
	if len(found) < 4 {
		panic(fmt.Errorf("unable to find BUSYBOX_IMAGE=<value> in %q", out))
	}
	busyboxTar = string(out[found[2]:found[3]])
	// Finally, check the file is present
	if _, err := os.Stat(busyboxTar); err != nil {
		panic(err)
	}
}

func ptrInt(v int) *int {
	return &v
}

func newStdBuffers() *stdBuffers {
	return &stdBuffers{
		Stdin:  bytes.NewBuffer(nil),
		Stdout: bytes.NewBuffer(nil),
		Stderr: bytes.NewBuffer(nil),
	}
}

type stdBuffers struct {
	Stdin  *bytes.Buffer
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

func (b *stdBuffers) String() string {
	s := []string{}
	if b.Stderr != nil {
		s = append(s, b.Stderr.String())
	}
	if b.Stdout != nil {
		s = append(s, b.Stdout.String())
	}
	return strings.Join(s, "|")
}

// ok fails the test if an err is not nil.
func ok(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func waitProcess(p *libcontainer.Process, t *testing.T) {
	t.Helper()
	status, err := p.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.Success() {
		t.Fatalf("unexpected status: %v", status)
	}
}

func newTestRoot() (string, error) {
	dir, err := ioutil.TempDir("", "libcontainer")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	testRoots = append(testRoots, dir)
	return dir, nil
}

func newTestBundle() (string, error) {
	dir, err := ioutil.TempDir("", "bundle")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// newRootfs creates a new tmp directory and copies the busybox root filesystem
func newRootfs() (string, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := copyBusybox(dir); err != nil {
		return "", err
	}
	return dir, nil
}

func remove(dir string) {
	_ = os.RemoveAll(dir)
}

// copyBusybox copies the rootfs for a busybox container created for the test image
// into the new directory for the specific test
func copyBusybox(dest string) error {
	out, err := exec.Command("sh", "-c", fmt.Sprintf("tar --exclude './dev/*' -C %q -xf %q", dest, busyboxTar)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("untar error %q: %q", err, out)
	}
	return nil
}

func newContainer(t *testing.T, config *configs.Config) (libcontainer.Container, error) {
	name := strings.ReplaceAll(t.Name(), "/", "_") + strconv.FormatInt(-int64(time.Now().Nanosecond()), 35)
	root, err := newTestRoot()
	if err != nil {
		return nil, err
	}

	f, err := libcontainer.New(root, libcontainer.Cgroupfs)
	if err != nil {
		return nil, err
	}
	if config.Cgroups != nil && config.Cgroups.Parent == "system.slice" {
		f, err = libcontainer.New(root, libcontainer.SystemdCgroups)
		if err != nil {
			return nil, err
		}
	}
	return f.Create(name, config)
}

// runContainer runs the container with the specific config and arguments
//
// buffers are returned containing the STDOUT and STDERR output for the run
// along with the exit code and any go error
func runContainer(t *testing.T, config *configs.Config, console string, args ...string) (buffers *stdBuffers, exitCode int, err error) {
	container, err := newContainer(t, config)
	if err != nil {
		return nil, -1, err
	}
	defer destroyContainer(container)
	buffers = newStdBuffers()
	process := &libcontainer.Process{
		Cwd:    "/",
		Args:   args,
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
		Init:   true,
	}

	err = container.Run(process)
	if err != nil {
		return buffers, -1, err
	}
	ps, err := process.Wait()
	if err != nil {
		return buffers, -1, err
	}
	status := ps.Sys().(syscall.WaitStatus)
	if status.Exited() {
		exitCode = status.ExitStatus()
	} else if status.Signaled() {
		exitCode = -int(status.Signal())
	} else {
		return buffers, -1, err
	}
	return
}

func destroyContainer(container libcontainer.Container) {
	_ = container.Destroy()
}
