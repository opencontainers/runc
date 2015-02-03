package integration

import (
	"log"
	"os"
	"runtime"

	"github.com/docker/libcontainer"
	_ "github.com/docker/libcontainer/nsenter"
)

// init runs the libcontainer initialization code because of the busybox style needs
// to work around the go runtime and the issues with forking
func init() {
	if len(os.Args) < 2 || os.Args[1] != "init" {
		return
	}
	runtime.LockOSThread()

	factory, err := libcontainer.New("", nil)
	if err != nil {
		log.Fatalf("unable to initialize for container: %s", err)
	}

	factory.StartInitialization(3)

	os.Exit(1)
}
