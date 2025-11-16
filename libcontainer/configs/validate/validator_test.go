package validate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

func TestValidate(t *testing.T) {
	config := &configs.Config{
		Rootfs: "/var",
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateWithInvalidRootfs(t *testing.T) {
	dir := "rootfs"
	if err := os.Symlink("/var", dir); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dir)

	config := &configs.Config{
		Rootfs: dir,
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateNetworkWithoutNETNamespace(t *testing.T) {
	network := &configs.Network{Type: "loopback"}
	config := &configs.Config{
		Rootfs:     "/var",
		Namespaces: []configs.Namespace{},
		Networks:   []*configs.Network{network},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateNetworkRoutesWithoutNETNamespace(t *testing.T) {
	route := &configs.Route{Gateway: "255.255.255.0"}
	config := &configs.Config{
		Rootfs:     "/var",
		Namespaces: []configs.Namespace{},
		Routes:     []*configs.Route{route},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateHostname(t *testing.T) {
	config := &configs.Config{
		Rootfs:   "/var",
		Hostname: "runc",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUTS},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateUTS(t *testing.T) {
	config := &configs.Config{
		Rootfs:     "/var",
		Domainname: "runc",
		Hostname:   "runc",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUTS},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateUTSWithoutUTSNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs:   "/var",
		Hostname: "runc",
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}

	config = &configs.Config{
		Rootfs:     "/var",
		Domainname: "runc",
	}

	err = Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateSecurityWithMaskPaths(t *testing.T) {
	config := &configs.Config{
		Rootfs:    "/var",
		MaskPaths: []string{"/proc/kcore"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWNS},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateSecurityWithROPaths(t *testing.T) {
	config := &configs.Config{
		Rootfs:        "/var",
		ReadonlyPaths: []string{"/proc/sys"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWNS},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateSecurityWithoutNEWNS(t *testing.T) {
	config := &configs.Config{
		Rootfs:        "/var",
		MaskPaths:     []string{"/proc/kcore"},
		ReadonlyPaths: []string{"/proc/sys"},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateUserNamespace(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); errors.Is(err, os.ErrNotExist) {
		t.Skip("Test requires userns.")
	}
	config := &configs.Config{
		Rootfs: "/var",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUSER},
			},
		),
		UIDMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
		GIDMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("expected error to not occur %+v", err)
	}
}

func TestValidateUsernsMappingWithoutNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs:      "/var",
		UIDMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
		GIDMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateTimeNamespace(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/time"); errors.Is(err, os.ErrNotExist) {
		t.Skip("Test requires timens.")
	}
	config := &configs.Config{
		Rootfs: "/var",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWTIME},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("expected error to not occur %+v", err)
	}
}

func TestValidateTimeNamespaceWithBothPathAndTimeOffset(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/time"); errors.Is(err, os.ErrNotExist) {
		t.Skip("Test requires timens.")
	}
	config := &configs.Config{
		Rootfs: "/var",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWTIME, Path: "/proc/1/ns/time"},
			},
		),
		TimeOffsets: map[string]specs.LinuxTimeOffset{
			"boottime":  {Secs: 150, Nanosecs: 314159},
			"monotonic": {Secs: 512, Nanosecs: 271818},
		},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateTimeOffsetsWithoutTimeNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs: "/var",
		TimeOffsets: map[string]specs.LinuxTimeOffset{
			"boottime":  {Secs: 150, Nanosecs: 314159},
			"monotonic": {Secs: 512, Nanosecs: 271818},
		},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

// TestConvertSysctlVariableToDotsSeparator tests whether the sysctl variable
// can be correctly converted to a dot as a separator.
func TestConvertSysctlVariableToDotsSeparator(t *testing.T) {
	type testCase struct {
		in  string
		out string
	}
	valid := []testCase{
		{in: "kernel.shm_rmid_forced", out: "kernel.shm_rmid_forced"},
		{in: "kernel/shm_rmid_forced", out: "kernel.shm_rmid_forced"},
		{in: "net.ipv4.conf.eno2/100.rp_filter", out: "net.ipv4.conf.eno2/100.rp_filter"},
		{in: "net/ipv4/conf/eno2.100/rp_filter", out: "net.ipv4.conf.eno2/100.rp_filter"},
		{in: "net/ipv4/ip_local_port_range", out: "net.ipv4.ip_local_port_range"},
		{in: "kernel/msgmax", out: "kernel.msgmax"},
		{in: "kernel/sem", out: "kernel.sem"},
	}

	for _, test := range valid {
		convertSysctlVal := convertSysctlVariableToDotsSeparator(test.in)
		if convertSysctlVal != test.out {
			t.Errorf("The sysctl variable was not converted correctly. got: %s, want: %s", convertSysctlVal, test.out)
		}
	}
}

func TestValidateSysctl(t *testing.T) {
	sysctl := map[string]string{
		"fs.mqueue.ctl":                    "ctl",
		"fs/mqueue/ctl":                    "ctl",
		"net.ctl":                          "ctl",
		"net/ctl":                          "ctl",
		"net.ipv4.conf.eno2/100.rp_filter": "ctl",
		"kernel.ctl":                       "ctl",
		"kernel/ctl":                       "ctl",
	}

	for k, v := range sysctl {
		config := &configs.Config{
			Rootfs: "/var",
			Sysctl: map[string]string{k: v},
		}

		err := Validate(config)
		if err == nil {
			t.Error("Expected error to occur but it was nil")
		}
	}
}

func TestValidateValidSysctl(t *testing.T) {
	sysctl := map[string]string{
		"fs.mqueue.ctl":                    "ctl",
		"fs/mqueue/ctl":                    "ctl",
		"net.ctl":                          "ctl",
		"net/ctl":                          "ctl",
		"net.ipv4.conf.eno2/100.rp_filter": "ctl",
		"kernel.msgmax":                    "ctl",
		"kernel/msgmax":                    "ctl",
	}

	for k, v := range sysctl {
		config := &configs.Config{
			Rootfs: "/var",
			Sysctl: map[string]string{k: v},
			Namespaces: []configs.Namespace{
				{
					Type: configs.NEWNET,
				},
				{
					Type: configs.NEWIPC,
				},
			},
		}

		err := Validate(config)
		if err != nil {
			t.Errorf("Expected error to not occur with {%s=%s} but got: %q", k, v, err)
		}
	}
}

func TestValidateSysctlWithSameNs(t *testing.T) {
	config := &configs.Config{
		Rootfs: "/var",
		Sysctl: map[string]string{"net.ctl": "ctl"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{
					Type: configs.NEWNET,
					Path: "/proc/self/ns/net",
				},
			},
		),
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateSysctlWithBindHostNetNS(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root")
	}

	const selfnet = "/proc/self/ns/net"

	file := filepath.Join(t.TempDir(), "default")
	fd, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file)
	fd.Close()

	if err := unix.Mount(selfnet, file, "bind", unix.MS_BIND, ""); err != nil {
		t.Fatalf("can't bind-mount %s to %s: %s", selfnet, file, err)
	}
	defer func() {
		_ = unix.Unmount(file, unix.MNT_DETACH)
	}()

	config := &configs.Config{
		Rootfs: "/var",
		Sysctl: map[string]string{"net.ctl": "ctl", "net.foo": "bar"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{
					Type: configs.NEWNET,
					Path: file,
				},
			},
		),
	}

	if err := Validate(config); err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateSysctlWithoutNETNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs:     "/var",
		Sysctl:     map[string]string{"net.ctl": "ctl"},
		Namespaces: []configs.Namespace{},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateMounts(t *testing.T) {
	testCases := []struct {
		isErr bool
		dest  string
	}{
		{isErr: false, dest: "not/an/abs/path"},
		{isErr: false, dest: "./rel/path"},
		{isErr: false, dest: "./rel/path"},
		{isErr: false, dest: "../../path"},
		{isErr: false, dest: "/abs/path"},
		{isErr: false, dest: "/abs/but/../unclean"},
	}

	for _, tc := range testCases {
		config := &configs.Config{
			Rootfs: "/var",
			Mounts: []*configs.Mount{
				{Destination: tc.dest},
			},
		}

		err := Validate(config)
		if tc.isErr && err == nil {
			t.Errorf("mount dest: %s, expected error, got nil", tc.dest)
		}
		if !tc.isErr && err != nil {
			t.Errorf("mount dest: %s, expected nil, got error %v", tc.dest, err)
		}
	}
}

func TestValidateBindMounts(t *testing.T) {
	testCases := []struct {
		isErr bool
		flags int
		data  string
	}{
		{isErr: false, flags: 0, data: ""},
		{isErr: false, flags: unix.MS_RDONLY | unix.MS_NOSYMFOLLOW, data: ""},

		{isErr: true, flags: 0, data: "idmap"},
		{isErr: true, flags: unix.MS_RDONLY, data: "custom_ext4_flag"},
		{isErr: true, flags: unix.MS_NOATIME, data: "rw=foobar"},
	}

	for _, tc := range testCases {
		for _, bind := range []string{"bind", "rbind"} {
			bindFlag := map[string]int{
				"bind":  unix.MS_BIND,
				"rbind": unix.MS_BIND | unix.MS_REC,
			}[bind]

			config := &configs.Config{
				Rootfs: "/var",
				Mounts: []*configs.Mount{
					{
						Destination: "/",
						Flags:       tc.flags | bindFlag,
						Data:        tc.data,
					},
				},
			}

			err := Validate(config)
			if tc.isErr && err == nil {
				t.Errorf("%s mount flags:0x%x data:%v, expected error, got nil", bind, tc.flags, tc.data)
			}
			if !tc.isErr && err != nil {
				t.Errorf("%s mount flags:0x%x data:%v, expected nil, got error %v", bind, tc.flags, tc.data, err)
			}
		}
	}
}

func TestValidateIDMapMounts(t *testing.T) {
	mapping := []configs.IDMap{
		{
			ContainerID: 0,
			HostID:      10000,
			Size:        1,
		},
	}

	testCases := []struct {
		name   string
		isErr  bool
		config *configs.Config
	}{
		{
			name:  "idmap non-bind mount",
			isErr: true,
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/dev/sda1",
						Destination: "/abs/path/",
						Device:      "ext4",
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name:  "idmap option non-bind mount",
			isErr: true,
			config: &configs.Config{
				Mounts: []*configs.Mount{
					{
						Source:      "/dev/sda1",
						Destination: "/abs/path/",
						Device:      "ext4",
						IDMapping:   &configs.MountIDMapping{},
					},
				},
			},
		},
		{
			name:  "ridmap option non-bind mount",
			isErr: true,
			config: &configs.Config{
				Mounts: []*configs.Mount{
					{
						Source:      "/dev/sda1",
						Destination: "/abs/path/",
						Device:      "ext4",
						IDMapping: &configs.MountIDMapping{
							Recursive: true,
						},
					},
				},
			},
		},
		{
			name:  "idmap mount no uid mapping",
			isErr: true,
			config: &configs.Config{
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name:  "idmap mount no gid mapping",
			isErr: true,
			config: &configs.Config{
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name:  "rootless idmap mount",
			isErr: true,
			config: &configs.Config{
				RootlessEUID: true,
				UIDMappings:  mapping,
				GIDMappings:  mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name: "idmap mounts without abs source path",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "./rel/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name: "idmap mounts without abs dest path",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "./rel/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name: "simple idmap mount",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/another-abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name: "idmap mount with more flags",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/another-abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND | unix.MS_RDONLY,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name: "idmap mount without userns mappings",
			config: &configs.Config{
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name: "idmap mounts with different userns and mount mappings",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: []configs.IDMap{
								{
									ContainerID: 10,
									HostID:      10,
									Size:        1,
								},
							},
							GIDMappings: mapping,
						},
					},
				},
			},
		},
		{
			name: "idmap mounts with different userns and mount mappings",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							UIDMappings: mapping,
							GIDMappings: []configs.IDMap{
								{
									ContainerID: 10,
									HostID:      10,
									Size:        1,
								},
							},
						},
					},
				},
			},
		},
		{
			name:  "mount with 'idmap' option but no mappings",
			isErr: true,
			config: &configs.Config{
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping:   &configs.MountIDMapping{},
					},
				},
			},
		},
		{
			name:  "mount with 'ridmap' option but no mappings",
			isErr: true,
			config: &configs.Config{
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						IDMapping: &configs.MountIDMapping{
							Recursive: true,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.config
			config.Rootfs = "/var"

			err := mountsStrict(config)
			if tc.isErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tc.isErr && err != nil {
				t.Error(err)
			}
		})
	}
}

func TestValidateScheduler(t *testing.T) {
	testCases := []struct {
		isErr     bool
		policy    string
		niceValue int32
		priority  int32
		runtime   uint64
		deadline  uint64
		period    uint64
	}{
		{isErr: true, niceValue: 0},
		{isErr: false, policy: "SCHED_OTHER", niceValue: 19},
		{isErr: false, policy: "SCHED_OTHER", niceValue: -20},
		{isErr: true, policy: "SCHED_OTHER", niceValue: 20},
		{isErr: true, policy: "SCHED_OTHER", niceValue: -21},
		{isErr: true, policy: "SCHED_OTHER", priority: 100},
		{isErr: false, policy: "SCHED_FIFO", priority: 100},
		{isErr: true, policy: "SCHED_FIFO", runtime: 20},
		{isErr: true, policy: "SCHED_BATCH", deadline: 30},
		{isErr: true, policy: "SCHED_IDLE", period: 40},
		{isErr: true, policy: "SCHED_DEADLINE", priority: 100},
		{isErr: false, policy: "SCHED_DEADLINE", runtime: 200},
		{isErr: false, policy: "SCHED_DEADLINE", deadline: 300},
		{isErr: false, policy: "SCHED_DEADLINE", period: 400},
		{isErr: true, policy: "SCHED_OTHER", niceValue: 20},
		{isErr: true, policy: "SCHED_OTHER", niceValue: -21},
		{isErr: false, policy: "SCHED_FIFO", priority: 100, niceValue: 100},
	}

	for _, tc := range testCases {
		scheduler := configs.Scheduler{
			Policy:   specs.LinuxSchedulerPolicy(tc.policy),
			Nice:     tc.niceValue,
			Priority: tc.priority,
			Runtime:  tc.runtime,
			Deadline: tc.deadline,
			Period:   tc.period,
		}
		config := &configs.Config{
			Rootfs:    "/var",
			Scheduler: &scheduler,
		}

		err := Validate(config)
		if tc.isErr && err == nil {
			t.Errorf("scheduler: %d, expected error, got nil", tc.niceValue)
		}
		if !tc.isErr && err != nil {
			t.Errorf("scheduler: %d, expected nil, got error %v", tc.niceValue, err)
		}
	}
}

func TestValidateIOPriority(t *testing.T) {
	testCases := []struct {
		isErr    bool
		priority int
		class    specs.IOPriorityClass
	}{
		{isErr: false, priority: 0, class: specs.IOPRIO_CLASS_IDLE},
		{isErr: false, priority: 7, class: specs.IOPRIO_CLASS_RT},
		{isErr: false, priority: 3, class: specs.IOPRIO_CLASS_BE},
		// Invalid priority.
		{isErr: true, priority: -1, class: specs.IOPRIO_CLASS_BE},
		// Invalid class.
		{isErr: true, priority: 3, class: specs.IOPriorityClass("IOPRIO_CLASS_WOW")},
	}

	for _, tc := range testCases {
		ioPriroty := configs.IOPriority{
			Priority: tc.priority,
			Class:    tc.class,
		}
		config := &configs.Config{
			Rootfs:     "/var",
			IOPriority: &ioPriroty,
		}

		err := Validate(config)
		if tc.isErr && err == nil {
			t.Errorf("iopriority: %d, expected error, got nil", tc.priority)
		}
		if !tc.isErr && err != nil {
			t.Errorf("iopriority: %d, expected nil, got error %v", tc.priority, err)
		}
	}
}

func TestValidateNetDevices(t *testing.T) {
	testCases := []struct {
		name   string
		isErr  bool
		config *configs.Config
	}{
		{
			name: "network device with configured network namespace",
			config: &configs.Config{
				Namespaces: configs.Namespaces(
					[]configs.Namespace{
						{
							Type: configs.NEWNET,
							Path: "/var/run/netns/blue",
						},
					},
				),
				NetDevices: map[string]*configs.LinuxNetDevice{
					"eth0": {},
				},
			},
		},
		{
			name: "network device rename",
			config: &configs.Config{
				Namespaces: configs.Namespaces(
					[]configs.Namespace{
						{
							Type: configs.NEWNET,
							Path: "/var/run/netns/blue",
						},
					},
				),
				NetDevices: map[string]*configs.LinuxNetDevice{
					"eth0": {
						Name: "c0",
					},
				},
			},
		},
		{
			name: "network device network namespace created by runc",
			config: &configs.Config{
				Namespaces: configs.Namespaces(
					[]configs.Namespace{
						{
							Type: configs.NEWNET,
						},
					},
				),
				NetDevices: map[string]*configs.LinuxNetDevice{
					"eth0": {},
				},
			},
		},
		{
			name:  "network device network namespace empty",
			isErr: true,
			config: &configs.Config{
				Namespaces: configs.Namespaces(
					[]configs.Namespace{},
				),
				NetDevices: map[string]*configs.LinuxNetDevice{
					"eth0": {},
				},
			},
		},
		{
			name:  "network device rootless EUID",
			isErr: true,
			config: &configs.Config{
				Namespaces: configs.Namespaces(
					[]configs.Namespace{
						{
							Type: configs.NEWNET,
							Path: "/var/run/netns/blue",
						},
					},
				),
				RootlessEUID: true,
				NetDevices: map[string]*configs.LinuxNetDevice{
					"eth0": {},
				},
			},
		},
		{
			name:  "network device rootless",
			isErr: true,
			config: &configs.Config{
				Namespaces: configs.Namespaces(
					[]configs.Namespace{
						{
							Type: configs.NEWNET,
							Path: "/var/run/netns/blue",
						},
					},
				),
				RootlessCgroups: true,
				NetDevices: map[string]*configs.LinuxNetDevice{
					"eth0": {},
				},
			},
		},
		{
			name:  "network device bad name",
			isErr: true,
			config: &configs.Config{
				Namespaces: configs.Namespaces(
					[]configs.Namespace{
						{
							Type: configs.NEWNET,
							Path: "/var/run/netns/blue",
						},
					},
				),
				NetDevices: map[string]*configs.LinuxNetDevice{
					"eth0": {
						Name: "eth0/",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.config
			config.Rootfs = "/var"

			err := Validate(config)
			if tc.isErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tc.isErr && err != nil {
				t.Error(err)
			}
		})
	}
}

func TestValidateUserSysctlWithUserNamespace(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); errors.Is(err, os.ErrNotExist) {
		t.Skip("Test requires userns.")
	}
	config := &configs.Config{
		Rootfs: "/var",
		Sysctl: map[string]string{"user.max_inotify_watches": "8192"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUSER},
			},
		),
		UIDMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
		GIDMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur with user.* sysctl and NEWUSER namespace: %+v", err)
	}
}

func TestValidateUserSysctlWithoutUserNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs: "/var",
		Sysctl: map[string]string{"user.max_inotify_watches": "8192"},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur with user.* sysctl without NEWUSER namespace but it was nil")
	}
}

func TestDevValidName(t *testing.T) {
	testCases := []struct {
		name  string
		valid bool
	}{
		{name: "", valid: false},
		{name: "a", valid: true},
		{name: strings.Repeat("a", unix.IFNAMSIZ), valid: true},
		{name: strings.Repeat("a", unix.IFNAMSIZ+1), valid: false},
		{name: ".", valid: false},
		{name: "..", valid: false},
		{name: "dev/null", valid: false},
		{name: "valid:name", valid: false},
		{name: "valid name", valid: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if devValidName(tc.name) != tc.valid {
				t.Fatalf("name %q, expected valid: %v", tc.name, tc.valid)
			}
		})
	}
}
