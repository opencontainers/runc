module github.com/opencontainers/runc

go 1.20

require (
	github.com/checkpoint-restore/go-criu/v6 v6.3.0
	github.com/cilium/ebpf v0.12.3
	github.com/containerd/console v1.0.3
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cyphar/filepath-securejoin v0.2.4
	github.com/docker/go-units v0.5.0
	github.com/godbus/dbus/v5 v5.1.0
	github.com/moby/sys/mountinfo v0.6.2
	github.com/moby/sys/user v0.1.0
	github.com/mrunalp/fileutils v0.5.1
	github.com/opencontainers/runtime-spec v1.1.1-0.20230823135140-4fec88fd00a4
	github.com/opencontainers/selinux v1.11.0
	github.com/seccomp/libseccomp-golang v0.10.0
	github.com/sirupsen/logrus v1.9.3
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/urfave/cli v1.22.12
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/net v0.19.0
	golang.org/x/sys v0.15.0
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df // indirect
	golang.org/x/exp v0.0.0-20230224173230-c95f2b4c22f2 // indirect
)
