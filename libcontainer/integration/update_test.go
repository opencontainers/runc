package integration

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/devices"
)

func testUpdateDevices(t *testing.T, systemd bool) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, &tParam{systemd: systemd})
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer func() {
		_ = stdinW.Close()
		if _, err := process.Wait(); err != nil {
			t.Log(err)
		}
	}()
	ok(t, err)

	var buf bytes.Buffer
	devCheck := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"/bin/sh", "-c", "echo > /dev/full; cat /dev/null; true"},
		Env:    standardEnvironment,
		Stderr: &buf,
	}
	isAllowed := true
	expected := map[bool][]string{
		true: {
			"write error: No space left on device", // from write to /dev/full
			// no error from cat /dev/null
		},
		false: {
			"/dev/full: Operation not permitted",
			`cat: can't open '/dev/null': Operation not permitted`,
		},
	}
	defaultDevices := config.Cgroups.Resources.Devices

	for i := 0; i < 300; i++ {
		// Check the access
		buf.Reset()
		err = container.Run(devCheck)
		ok(t, err)
		waitProcess(devCheck, t)

		for _, exp := range expected[isAllowed] {
			if !strings.Contains(buf.String(), exp) {
				t.Fatalf("[%d] expected %q, got %q", i, exp, buf.String())
			}
		}

		// Now flip the access permission
		isAllowed = !isAllowed
		if isAllowed {
			config.Cgroups.Resources.Devices = defaultDevices
		} else {
			config.Cgroups.Resources.Devices = []*devices.Rule{}
		}
		if err := container.Set(*config); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUpdateDevices(t *testing.T) {
	testUpdateDevices(t, false)
}

func TestUpdateDevicesSystemd(t *testing.T) {
	if !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	testUpdateDevices(t, true)
}
