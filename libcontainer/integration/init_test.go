package integration

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

// init runs the libcontainer initialization code because of the busybox style needs
// to work around the go runtime and the issues with forking
func init() {
	if len(os.Args) < 2 || os.Args[1] != "init" {
		return
	}
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
	factory, err := libcontainer.New("")
	if err != nil {
		logrus.Fatalf("unable to initialize for container: %s", err)
	}
	if err := factory.StartInitialization(); err != nil {
		// return proper unix error codes
		if exerr, ok := err.(*exec.Error); ok {
			switch exerr.Err {
			case os.ErrPermission:
				fmt.Fprintln(os.Stderr, err)
				os.Exit(126)
			case exec.ErrNotFound:
				fmt.Fprintln(os.Stderr, err)
				os.Exit(127)
			default:
				if os.IsNotExist(exerr.Err) {
					fmt.Fprintf(os.Stderr, "exec: %s: %v\n", strconv.Quote(exerr.Name), os.ErrNotExist)
					os.Exit(127)
				}
			}
		}
		logrus.Fatal(err)
	}
	panic("init: init failed to start contianer")
}

var (
	factory        libcontainer.Factory
	systemdFactory libcontainer.Factory
)

func TestMain(m *testing.M) {
	var (
		err error
		ret int = 0
	)

	logrus.SetOutput(os.Stderr)
	logrus.SetLevel(logrus.InfoLevel)

	factory, err = libcontainer.New(".", libcontainer.Cgroupfs)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	if systemd.UseSystemd() {
		systemdFactory, err = libcontainer.New(".", libcontainer.SystemdCgroups)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
	}

	ret = m.Run()
	os.Exit(ret)
}
