// +build linux

package libcontainer

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"fmt"
	"github.com/docker/docker/pkg/mount"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"
)

func newTestRoot() (string, error) {
	dir, err := ioutil.TempDir("", "libcontainer")
	if err != nil {
		return "", err
	}
	return dir, nil
}

func checkFactory(factory Factory, root string) (*LinuxFactory, error) {
	if factory == nil {
		return nil, errors.New("factory should not be nil")
	}

	lfactory, ok := factory.(*LinuxFactory)
	if !ok {
		return nil, errors.New("expected linux factory returned on linux based systems")
	}

	if lfactory.Root != root {
		return lfactory, errors.New(fmt.Sprintf("expected factory root to be %q but received %q", root, lfactory.Root))
	}

	if factory.Type() != "libcontainer" {
		return lfactory, errors.New(fmt.Sprintf("unexpected factory type: %q, expected %q", factory.Type(), "libcontainer"))
	}

	return lfactory, nil
}

func TestFactoryNew(t *testing.T) {
	root, rerr := newTestRoot()
	if rerr != nil {
		t.Fatal(rerr)
	}
	defer os.RemoveAll(root)
	factory, err := New(root, Cgroupfs)
	if err != nil {
		t.Fatal(err)
	}
	_, err1 := checkFactory(factory, root)
	if err1 != nil {
		t.Fatal(err1)
	}
}

func TestNewCriuPath(t *testing.T) {
	root, rerr := newTestRoot()
	if rerr != nil {
		t.Fatal(rerr)
	}
	defer os.RemoveAll(root)
	factory, err := New(root, Cgroupfs, CriuPath("criu"))
	if err != nil {
		t.Fatal(err)
	}
	lfactory, err := checkFactory(factory, root)
	if err != nil {
		t.Fatal(err)
	}
	if lfactory.CriuPath != "criu" {
		t.Fatal(fmt.Sprintf("unexpected factory CriuPath: %q, expected %q", lfactory.CriuPath, "criu"))
	}
}

func TestFactoryNewTmpfs(t *testing.T) {
	root, rerr := newTestRoot()
	if rerr != nil {
		t.Fatal(rerr)
	}
	defer os.RemoveAll(root)
	factory, err := New(root, Cgroupfs, TmpfsRoot)
	if err != nil {
		t.Fatal(err)
	}

	lfactory, err := checkFactory(factory, root)
	if err != nil {
		t.Fatal(err)
	}
	mounted, err := mount.Mounted(lfactory.Root)
	if err != nil {
		t.Fatal(err)
	}
	if !mounted {
		t.Fatalf("Factory Root is not mounted")
	}
	defer syscall.Unmount(root, syscall.MNT_DETACH)
	mounts, err := mount.GetMounts()
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, m := range mounts {
		if m.Mountpoint == lfactory.Root {
			if m.Fstype != "tmpfs" {
				t.Fatalf("Fstype of root: %s, expected %s", m.Fstype, "tmpfs")
			}
			if m.Source != "tmpfs" {
				t.Fatalf("Source of root: %s, expected %s", m.Source, "tmpfs")
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("Factory Root is not listed in mounts list")
	}

}

func TestFactoryLoadNotExists(t *testing.T) {
	root, rerr := newTestRoot()
	if rerr != nil {
		t.Fatal(rerr)
	}
	defer os.RemoveAll(root)
	factory, err := New(root, Cgroupfs)
	if err != nil {
		t.Fatal(err)
	}
	_, err = factory.Load("nocontainer")
	if err == nil {
		t.Fatal("expected nil error loading non-existing container")
	}

	lerr, ok := err.(Error)
	if !ok {
		t.Fatal("expected libcontainer error type")
	}
	if lerr.Code() != ContainerNotExists {
		t.Fatalf("expected error code %s but received %s", ContainerNotExists, lerr.Code())
	}
}

func TestFactoryLoadContainer(t *testing.T) {
	root, err := newTestRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	// setup default container config and state for mocking
	var (
		id            = "1"
		expectedHooks = &configs.Hooks{
			Prestart: []configs.Hook{
				configs.CommandHook{Command: configs.Command{Path: "prestart-hook"}},
			},
			Poststart: []configs.Hook{
				configs.CommandHook{Command: configs.Command{Path: "poststart-hook"}},
			},
			Poststop: []configs.Hook{
				unserializableHook{},
				configs.CommandHook{Command: configs.Command{Path: "poststop-hook"}},
			},
		}
		expectedConfig = &configs.Config{
			Rootfs: "/mycontainer/root",
			Hooks:  expectedHooks,
		}
		expectedState = &State{
			BaseState: BaseState{
				InitProcessPid: 1024,
				Config:         *expectedConfig,
			},
		}
	)
	if err := os.Mkdir(filepath.Join(root, id), 0700); err != nil {
		t.Fatal(err)
	}
	if err := marshal(filepath.Join(root, id, stateFilename), expectedState); err != nil {
		t.Fatal(err)
	}
	factory, err := New(root, Cgroupfs)
	if err != nil {
		t.Fatal(err)
	}
	container, err := factory.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if container.ID() != id {
		t.Fatalf("expected container id %q but received %q", id, container.ID())
	}
	config := container.Config()
	if config.Rootfs != expectedConfig.Rootfs {
		t.Fatalf("expected rootfs %q but received %q", expectedConfig.Rootfs, config.Rootfs)
	}
	expectedHooks.Poststop = expectedHooks.Poststop[1:] // expect unserializable hook to be skipped
	if !reflect.DeepEqual(config.Hooks, expectedHooks) {
		t.Fatalf("expects hooks %q but received %q", expectedHooks, config.Hooks)
	}
	lcontainer, ok := container.(*linuxContainer)
	if !ok {
		t.Fatal("expected linux container on linux based systems")
	}
	if lcontainer.initProcess.pid() != expectedState.InitProcessPid {
		t.Fatalf("expected init pid %d but received %d", expectedState.InitProcessPid, lcontainer.initProcess.pid())
	}
}

func marshal(path string, v interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return utils.WriteJSON(f, v)
}

type unserializableHook struct{}

func (unserializableHook) Run(configs.HookState) error {
	return nil
}
