module github.com/opencontainers/runc

go 1.25.0

require (
	cyphar.com/go-pathrs v0.2.5-0.20260521202425-893bcf88f601
	github.com/checkpoint-restore/go-criu/v7 v7.2.0
	github.com/containerd/console v1.0.5
	github.com/coreos/go-systemd/v22 v22.7.0
	github.com/cyphar/filepath-securejoin v0.6.1
	github.com/docker/go-units v0.5.0
	github.com/godbus/dbus/v5 v5.2.2
	github.com/moby/sys/capability v0.4.0
	github.com/moby/sys/devices v0.1.0
	github.com/moby/sys/mountinfo v0.7.2
	github.com/moby/sys/user v0.4.0
	github.com/moby/sys/userns v0.1.0
	github.com/mrunalp/fileutils v0.5.1
	github.com/opencontainers/cgroups v0.0.6
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/opencontainers/selinux v1.14.1
	github.com/seccomp/libseccomp-golang v0.11.1
	github.com/sirupsen/logrus v1.9.4
	github.com/urfave/cli/v3 v3.9.0
	github.com/vishvananda/netlink v1.3.1
	github.com/vishvananda/netns v0.0.5
	golang.org/x/net v0.54.0
	golang.org/x/sys v0.44.0
	google.golang.org/protobuf v1.36.11
)

require github.com/cilium/ebpf v0.17.3 // indirect
