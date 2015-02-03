package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/configs"
)

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

func writeConfig(config *configs.Config) error {
	f, err := os.OpenFile(filepath.Join(config.RootFs, "container.json"), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(config)
}

func loadConfig() (*configs.Config, error) {
	f, err := os.Open(filepath.Join(os.Getenv("data_path"), "container.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var container *configs.Config
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		return nil, err
	}
	return container, nil
}

// newRootFs creates a new tmp directory and copies the busybox root filesystem
func newRootFs() (string, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := copyBusybox(dir); err != nil {
		return "", nil
	}
	return dir, nil
}

func remove(dir string) {
	os.RemoveAll(dir)
}

// copyBusybox copies the rootfs for a busybox container created for the test image
// into the new directory for the specific test
func copyBusybox(dest string) error {
	out, err := exec.Command("sh", "-c", fmt.Sprintf("cp -R /busybox/* %s/", dest)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy error %q: %q", err, out)
	}
	return nil
}

// runContainer runs the container with the specific config and arguments
//
// buffers are returned containing the STDOUT and STDERR output for the run
// along with the exit code and any go error
func runContainer(config *configs.Config, console string, args ...string) (buffers *stdBuffers, exitCode int, err error) {
	if err := writeConfig(config); err != nil {
		return nil, -1, err
	}

	buffers = newStdBuffers()

	process := &libcontainer.Process{
		Args:   args,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}

	factory, err := libcontainer.New(".", []string{os.Args[0], "init", "--"})
	if err != nil {
		return nil, -1, err
	}

	container, err := factory.Create("testCT", config)
	if err != nil {
		return nil, -1, err
	}
	defer container.Destroy()

	pid, err := container.Start(process)
	if err != nil {
		return nil, -1, err
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return nil, -1, err
	}

	ps, err := p.Wait()
	if err != nil {
		return nil, -1, err
	}

	status := ps.Sys().(syscall.WaitStatus)
	if status.Exited() {
		exitCode = status.ExitStatus()
	} else if status.Signaled() {
		exitCode = -int(status.Signal())
	} else {
		return nil, -1, err
	}

	return
}
