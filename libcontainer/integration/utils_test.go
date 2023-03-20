package integration

import (
	"bytes"
	"fmt"
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
		panic(fmt.Errorf("getImages error %w (output: %s)", err, out))
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

// newRootfs creates a new tmp directory and copies the busybox root
// filesystem to it.
func newRootfs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := copyBusybox(dir); err != nil {
		t.Fatal(err)
	}

	// Make sure others can read+exec, so all tests (inside userns too) can
	// read the rootfs.
	if err := traversePath(dir); err != nil {
		t.Fatalf("Error making newRootfs path traversable by others: %v", err)
	}

	return dir
}

// traversePath gives read+execute permissions to others for all elements in tPath below
// os.TempDir() and errors out if elements above it don't have read+exec permissions for others.
// tPath MUST be a descendant of os.TempDir(). The path returned by testing.TempDir() usually is.
func traversePath(tPath string) error {
	// Check the assumption that the argument is under os.TempDir().
	tempBase := os.TempDir()
	if !strings.HasPrefix(tPath, tempBase) {
		return fmt.Errorf("traversePath: %q is not a descendant of %q", tPath, tempBase)
	}

	var path string
	for _, p := range strings.SplitAfter(tPath, "/") {
		path = path + p
		stats, err := os.Stat(path)
		if err != nil {
			return err
		}

		perm := stats.Mode().Perm()

		if perm&0o5 == 0o5 {
			continue
		}

		if strings.HasPrefix(tempBase, path) {
			return fmt.Errorf("traversePath: directory %q MUST have read+exec permissions for others", path)
		}

		if err := os.Chmod(path, perm|0o5); err != nil {
			return err
		}
	}

	return nil
}

func remove(dir string) {
	_ = os.RemoveAll(dir)
}

// copyBusybox copies the rootfs for a busybox container created for the test image
// into the new directory for the specific test
func copyBusybox(dest string) error {
	out, err := exec.Command("sh", "-c", fmt.Sprintf("tar --exclude './dev/*' -C %q -xf %q", dest, busyboxTar)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("untar error %w: %q", err, out)
	}
	return nil
}

func newContainer(t *testing.T, config *configs.Config) (*libcontainer.Container, error) {
	name := strings.ReplaceAll(t.Name(), "/", "_") + strconv.FormatInt(-int64(time.Now().Nanosecond()), 35)
	root := t.TempDir()

	return libcontainer.Create(root, name, config)
}

// runContainer runs the container with the specific config and arguments
//
// buffers are returned containing the STDOUT and STDERR output for the run
// along with the exit code and any go error
func runContainer(t *testing.T, config *configs.Config, args ...string) (buffers *stdBuffers, exitCode int, err error) {
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

// runContainerOk is a wrapper for runContainer, simplifying its use for cases
// when the run is expected to succeed and return exit code of 0.
func runContainerOk(t *testing.T, config *configs.Config, args ...string) *stdBuffers {
	buffers, exitCode, err := runContainer(t, config, args...)

	t.Helper()
	if err != nil {
		t.Fatalf("%s: %s", buffers, err)
	}
	if exitCode != 0 {
		t.Fatalf("exit code not 0. code %d stderr %q", exitCode, buffers.Stderr)
	}

	return buffers
}

func destroyContainer(container *libcontainer.Container) {
	_ = container.Destroy()
}
