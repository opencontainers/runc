module github.com/opencontainers/runc

go 1.17

require (
	github.com/checkpoint-restore/go-criu/v5 v5.3.0
	github.com/cilium/ebpf v0.9.1
	github.com/containerd/console v1.0.3
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/docker/go-units v0.4.0
	github.com/godbus/dbus/v5 v5.1.0
	github.com/moby/sys/mountinfo v0.6.2
	github.com/mrunalp/fileutils v0.5.0
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/selinux v1.10.1
	github.com/seccomp/libseccomp-golang v0.10.0
	github.com/sirupsen/logrus v1.8.1
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/urfave/cli v1.22.9
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/net v0.0.0-20201224014010-6772e930b67b
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a
	google.golang.org/protobuf v1.28.0
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.0-20190314233015-f79a8a8ca69d // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df // indirect
)
