package configs

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

// BaseConfig defines the platform agnostic configuration options for executing
// a process inside a contained environment.
type BaseConfig struct {
	// Path to a directory containing the container's root filesystem.
	Rootfs string `json:"rootfs"`

	// Mounts specify additional source and destination paths that will be mounted inside the container's
	// rootfs and mount namespace if specified
	Mounts []*Mount `json:"mounts"`

	// Hostname optionally sets the container's hostname if provided
	Hostname string `json:"hostname"`

	// Hooks are a collection of actions to perform at various container lifecycle events.
	// Hooks are not able to be marshaled to json but they are also not needed to.
	Hooks *Hooks `json:"-"`

	// Version is the version of opencontainer specification that is supported.
	Version string `json:"version"`
}

type Hooks struct {
	// Prestart commands are executed after the container namespaces are created,
	// but before the user supplied command is executed from init.
	Prestart []Hook

	// Poststop commands are executed after the container init process exits.
	Poststop []Hook
}

// HookState is the payload provided to a hook on execution.
type HookState struct {
	Version string `json:"version"`
	ID      string `json:"id"`
	Pid     int    `json:"pid"`
	Root    string `json:"root"`
}

type Hook interface {
	// Run executes the hook with the provided state.
	Run(HookState) error
}

// NewFunctionHooks will call the provided function when the hook is run.
func NewFunctionHook(f func(HookState) error) FuncHook {
	return FuncHook{
		run: f,
	}
}

type FuncHook struct {
	run func(HookState) error
}

func (f FuncHook) Run(s HookState) error {
	return f.run(s)
}

type Command struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
	Env  []string `json:"env"`
	Dir  string   `json:"dir"`
}

// NewCommandHooks will execute the provided command when the hook is run.
func NewCommandHook(cmd Command) CommandHook {
	return CommandHook{
		Command: cmd,
	}
}

type CommandHook struct {
	Command
}

func (c Command) Run(s HookState) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	cmd := exec.Cmd{
		Path:  c.Path,
		Args:  c.Args,
		Env:   c.Env,
		Stdin: bytes.NewReader(b),
	}
	return cmd.Run()
}
