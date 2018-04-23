package api

import (
	"errors"
	"io"
	"syscall"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var (
	ErrNotImplemented = errors.New("command not implemented")
	ErrEmptyID        = errors.New("container id cannot be empty")
)

type API interface {
	Checkpoint(id string, opts CheckpointOpts) error
	Create(id string, opts CreateOpts) (*CreateResult, error)
	Delete(id string, opts DeleteOpts) error
	Kill(id string, sig syscall.Signal, opts KillOpts) error
	List() ([]Container, error)
	PS(id string) ([]int, error)
	Pause(id string) error
	Restore(id string, opts RestoreOpts) (*CreateResult, error)
	Resume(id string) error
	Run(id string, opts CreateOpts) (*CreateResult, error)
	Start(id string) error
	State(id string) (*Container, error)
	Exec(id string, opts ExecOpts) (*CreateResult, error)
}

type ExecOpts struct {
	PidFile       string
	Detach        bool
	ConsoleSocket string
	Process       *specs.Process
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
}

type RestoreOpts struct {
	CreateOpts
	CheckpointOpts
}

type DeleteOpts struct {
	Force bool
}

type KillOpts struct {
	All bool
}

type CriuPageServerInfo struct {
	Address string // IP address of CRIU page server
	Port    int32  // port number of CRIU page server
}

type CheckpointOpts struct {
	ImagesDirectory         string             // directory for storing image files
	WorkDirectory           string             // directory to cd and write logs/pidfiles/stats to
	ParentImage             string             // directory for storing parent image files in pre-dump and dump
	LeaveRunning            bool               // leave container in running state after checkpoint
	TcpEstablished          bool               // checkpoint/restore established TCP connections
	ExternalUnixConnections bool               // allow external unix connections
	ShellJob                bool               // allow to dump and restore shell jobs
	FileLocks               bool               // handle file locks, for safety
	PreDump                 bool               // call criu predump to perform iterative checkpoint
	PageServer              CriuPageServerInfo // allow to dump to criu page server
	ManageCgroupsMode       string             // dump or restore cgroup mode
	EmptyNs                 uint32             // don't c/r properties for namespace from this mask
	AutoDedup               bool               // auto deduplication for incremental dumps
	LazyPages               bool               // restore memory pages lazily using userfaultfd
	StatusFd                string             // fd for feedback when lazy server is ready
}

type CreateOpts struct {
	Spec          *specs.Spec
	PidFile       string
	ConsoleSocket string
	NoPivot       bool
	NoNewKeyring  bool
	PreserveFDs   int
	Detach        bool
	NoSubreaper   bool
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
}

type CreateResult struct {
	Status int
}

// Container represents the platform agnostic pieces relating to a
// running container's status and state
type Container struct {
	// Version is the OCI version for the container
	Version string `json:"ociVersion"`
	// ID is the container ID
	ID string `json:"id"`
	// InitProcessPid is the init process id in the parent namespace
	InitProcessPid int `json:"pid"`
	// Status is the current status of the container, running, paused, ...
	Status string `json:"status"`
	// Bundle is the path on the filesystem to the bundle
	Bundle string `json:"bundle"`
	// Rootfs is a path to a directory containing the container's root filesystem.
	Rootfs string `json:"rootfs"`
	// Created is the unix timestamp for the creation time of the container in UTC
	Created time.Time `json:"created"`
	// Annotations is the user defined annotations added to the config.
	Annotations map[string]string `json:"annotations,omitempty"`
	// The owner of the state directory (the owner of the container).
	Owner string `json:"owner"`
}
