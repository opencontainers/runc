package linux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/opencontainers/runc/api"
	"github.com/opencontainers/runc/libcontainer"
	"golang.org/x/sys/unix"
)

func (l *Libcontainer) Delete(ctx context.Context, id string, opts api.DeleteOpts) error {
	container, err := l.getContainer(id)
	if err != nil {
		if lerr, ok := err.(libcontainer.Error); ok && lerr.Code() == libcontainer.ContainerNotExists {
			// if there was an aborted start or something of the sort then the container's directory could exist but
			// libcontainer does not see it because the state.json file inside that directory was never created.
			path := filepath.Join(l.root, id)
			if e := os.RemoveAll(path); e != nil {
				fmt.Fprintf(os.Stderr, "remove %s: %v\n", path, e)
			}
			if opts.Force {
				return nil
			}
		}
		return err
	}
	s, err := container.Status()
	if err != nil {
		return err
	}
	switch s {
	case libcontainer.Stopped:
		destroy(container)
	case libcontainer.Created:
		return killContainer(container)
	default:
		if opts.Force {
			return killContainer(container)
		} else {
			return fmt.Errorf("cannot delete container %s that is not stopped: %s\n", id, s)
		}
	}
	return nil
}

func killContainer(container libcontainer.Container) error {
	_ = container.Signal(unix.SIGKILL, false)
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := container.Signal(syscall.Signal(0), false); err != nil {
			destroy(container)
			return nil
		}
	}
	return fmt.Errorf("container init still running")
}
