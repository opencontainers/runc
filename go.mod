module github.com/opencontainers/runc

go 1.19

require (
	github.com/checkpoint-restore/go-criu/v6 v6.3.0
	github.com/cilium/ebpf v0.10.0
	github.com/containerd/console v1.0.3
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/docker/go-units v0.5.0
	github.com/godbus/dbus/v5 v5.1.0
	github.com/moby/sys/mountinfo v0.6.2
	github.com/mrunalp/fileutils v0.5.0
	github.com/opencontainers/runtime-spec v1.1.0-rc.1
	github.com/opencontainers/selinux v1.11.0
	github.com/seccomp/libseccomp-golang v0.10.0
	github.com/sirupsen/logrus v1.9.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/urfave/cli v1.22.9
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/net v0.7.0
	golang.org/x/sys v0.5.0
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df // indirect
)
