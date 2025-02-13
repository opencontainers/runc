module github.com/opencontainers/runc

go 1.22

// Suggest toolchain 1.22.4 due to a fix in golang for libcontainer/nsenter/.
// For more info, see: #4233
// Note that toolchain does not impose a requirement on other modules using runc.
toolchain go1.22.4

require (
	github.com/checkpoint-restore/go-criu/v6 v6.3.0
	github.com/cilium/ebpf v0.17.3
	github.com/containerd/console v1.0.4
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cyphar/filepath-securejoin v0.4.1
	github.com/docker/go-units v0.5.0
	github.com/godbus/dbus/v5 v5.1.0
	github.com/moby/sys/capability v0.4.0
	github.com/moby/sys/mountinfo v0.7.2
	github.com/moby/sys/user v0.3.0
	github.com/moby/sys/userns v0.1.0
	github.com/mrunalp/fileutils v0.5.1
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/opencontainers/selinux v1.11.1
	github.com/seccomp/libseccomp-golang v0.10.0
	github.com/sirupsen/logrus v1.9.3
	github.com/urfave/cli v1.22.16
	github.com/vishvananda/netlink v1.3.0
	golang.org/x/net v0.35.0
	golang.org/x/sys v0.30.0
	google.golang.org/protobuf v1.36.5
)

// https://github.com/opencontainers/runc/issues/4594
exclude (
	github.com/cilium/ebpf v0.17.0
	github.com/cilium/ebpf v0.17.1
	github.com/cilium/ebpf v0.17.2
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.5 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
)
