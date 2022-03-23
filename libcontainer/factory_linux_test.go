package libcontainer

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestFactoryLoadNotExists(t *testing.T) {
	stateDir := t.TempDir()
	_, err := Load(stateDir, "nocontainer")
	if err == nil {
		t.Fatal("expected nil error loading non-existing container")
	}
	if !errors.Is(err, ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestFactoryLoadContainer(t *testing.T) {
	root := t.TempDir()
	// setup default container config and state for mocking
	var (
		id            = "1"
		expectedHooks = configs.Hooks{
			configs.Prestart: configs.HookList{
				configs.CommandHook{Command: configs.Command{Path: "prestart-hook"}},
			},
			configs.Poststart: configs.HookList{
				configs.CommandHook{Command: configs.Command{Path: "poststart-hook"}},
			},
			configs.Poststop: configs.HookList{
				unserializableHook{},
				configs.CommandHook{Command: configs.Command{Path: "poststop-hook"}},
			},
		}
		expectedConfig = &configs.Config{
			Rootfs: "/mycontainer/root",
			Hooks:  expectedHooks,
			Cgroups: &configs.Cgroup{
				Resources: &configs.Resources{},
			},
		}
		expectedState = &State{
			BaseState: BaseState{
				InitProcessPid: 1024,
				Config:         *expectedConfig,
			},
		}
	)
	if err := os.Mkdir(filepath.Join(root, id), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := marshal(filepath.Join(root, id, stateFilename), expectedState); err != nil {
		t.Fatal(err)
	}
	container, err := Load(root, id)
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
	expectedHooks[configs.Poststop] = expectedHooks[configs.Poststop][1:] // expect unserializable hook to be skipped
	if !reflect.DeepEqual(config.Hooks, expectedHooks) {
		t.Fatalf("expects hooks %q but received %q", expectedHooks, config.Hooks)
	}
	if container.initProcess.pid() != expectedState.InitProcessPid {
		t.Fatalf("expected init pid %d but received %d", expectedState.InitProcessPid, container.initProcess.pid())
	}
}

func marshal(path string, v interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint: errcheck
	return utils.WriteJSON(f, v)
}

type unserializableHook struct{}

func (unserializableHook) Run(*specs.State) error {
	return nil
}
