package runc

import (
	"github.com/opencontainers/runc/libcontainer/configs"
)

// The SpecConfig Interface is implemented by platform specific Spec
// which allow updating the container config with the Spec settings.
type SpecConfig interface {
	// Construct a new cotainer config based on Spec data
	NewConfig() (*configs.Config, error)
	// Add namespaces in Spec to the container
	AddNamepsaces(config *configs.Config) error
	// Add mounts in the Spec to the container
	AddMounts(config *configs.Config) error
	// Add devices in the Spec to the conatiner
	AddDevices(config *configs.Config) error
	// Add the user namespace in the Spec to the container
	AddUserNamespace(config *configs.Config) error
	// Add the Cgroups based on the configured devices
	AddGroups(config *configs.Config) error
	// Update mounts if configured filesystem is read only
	SetReadOnly(config *configs.Config) error
	// The current cpu quota based on the platform multiplier
	CPUQuota() int64
}

// Spec provides basic properties to apply on platforms
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

// Mount holds the properties for configure mount points
type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Options     string `json:"options"`
}

// Process holds the properties for processes run in the container
type Process struct {
	TTY  bool     `json:"tty"`
	User string   `json:"user"`
	Args []string `json:"args"`
	Env  []string `json:"env"`
	Cwd  string   `json:"cwd"`
}

// Root holds the properties of the filesystem
type Root struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly"`
}

// Namespace defines the basic properties of the container
type Namespace struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}
