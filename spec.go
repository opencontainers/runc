package runc

import (
	"github.com/docker/libcontainer/configs"
)

type SpecConfig interface {
	NewConfig() (*configs.Config, error)
	AddNamepsaces(config *configs.Config) error
	AddMounts(config *configs.Config) error
	AddDevices(config *configs.Config) error
	AddUserNamespace(config *configs.Config) error
	AddGroups(config *configs.Config) error
	SetReadOnly(config *configs.Config) error
}

type Spec struct {
	Version      string      `json:"version"`
	OS           string      `json:"os"`
	Arch         string      `json:"arch"`
	Processes    []*Process  `json:"processes"`
	Root         Root        `json:"root"`
	Cpus         float64     `json:"cpus"`   // in 1.1 for 110% cpus
	Memory       int64       `json:"memory"` // in mb; 1024m
	Hostname     string      `json:"hostname"`
	Namespaces   []Namespace `json:"namespaces"`
	Capabilities []string    `json:"capabilities"`
	Devices      []string    `json:"devices"`
	Mounts       []Mount     `json:"mounts"`
}

type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Options     string `json:"options"`
}

type Process struct {
	TTY  bool     `json:"tty"`
	User string   `json:"user"`
	Args []string `json:"args"`
	Env  []string `json:"env"`
	Cwd  string   `json:"cwd"`
}

type Root struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly"`
}

type Namespace struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}
