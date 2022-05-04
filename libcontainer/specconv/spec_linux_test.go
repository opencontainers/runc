package specconv

import (
	"os"
	"strings"
	"testing"

	dbus "github.com/godbus/dbus/v5"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/configs/validate"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

func TestCreateCommandHookTimeout(t *testing.T) {
	timeout := 3600
	hook := specs.Hook{
		Path:    "/some/hook/path",
		Args:    []string{"--some", "thing"},
		Env:     []string{"SOME=value"},
		Timeout: &timeout,
	}
	command := createCommandHook(hook)
	timeoutStr := command.Timeout.String()
	if timeoutStr != "1h0m0s" {
		t.Errorf("Expected the Timeout to be 1h0m0s, got: %s", timeoutStr)
	}
}

func TestCreateHooks(t *testing.T) {
	rspec := &specs.Spec{
		Hooks: &specs.Hooks{
			Prestart: []specs.Hook{
				{
					Path: "/some/hook/path",
				},
				{
					Path: "/some/hook2/path",
					Args: []string{"--some", "thing"},
				},
			},
			CreateRuntime: []specs.Hook{
				{
					Path: "/some/hook/path",
				},
				{
					Path: "/some/hook2/path",
					Args: []string{"--some", "thing"},
				},
			},
			CreateContainer: []specs.Hook{
				{
					Path: "/some/hook/path",
				},
				{
					Path: "/some/hook2/path",
					Args: []string{"--some", "thing"},
				},
			},
			StartContainer: []specs.Hook{
				{
					Path: "/some/hook/path",
				},
				{
					Path: "/some/hook2/path",
					Args: []string{"--some", "thing"},
				},
			},
			Poststart: []specs.Hook{
				{
					Path: "/some/hook/path",
					Args: []string{"--some", "thing"},
					Env:  []string{"SOME=value"},
				},
				{
					Path: "/some/hook2/path",
				},
				{
					Path: "/some/hook3/path",
				},
			},
			Poststop: []specs.Hook{
				{
					Path: "/some/hook/path",
					Args: []string{"--some", "thing"},
					Env:  []string{"SOME=value"},
				},
				{
					Path: "/some/hook2/path",
				},
				{
					Path: "/some/hook3/path",
				},
				{
					Path: "/some/hook4/path",
					Args: []string{"--some", "thing"},
				},
			},
		},
	}
	conf := &configs.Config{}
	createHooks(rspec, conf)

	prestart := conf.Hooks[configs.Prestart]

	if len(prestart) != 2 {
		t.Error("Expected 2 Prestart hooks")
	}

	createRuntime := conf.Hooks[configs.CreateRuntime]

	if len(createRuntime) != 2 {
		t.Error("Expected 2 createRuntime hooks")
	}

	createContainer := conf.Hooks[configs.CreateContainer]

	if len(createContainer) != 2 {
		t.Error("Expected 2 createContainer hooks")
	}

	startContainer := conf.Hooks[configs.StartContainer]

	if len(startContainer) != 2 {
		t.Error("Expected 2 startContainer hooks")
	}

	poststart := conf.Hooks[configs.Poststart]

	if len(poststart) != 3 {
		t.Error("Expected 3 Poststart hooks")
	}

	poststop := conf.Hooks[configs.Poststop]

	if len(poststop) != 4 {
		t.Error("Expected 4 Poststop hooks")
	}
}

func TestSetupSeccompNil(t *testing.T) {
	seccomp, err := SetupSeccomp(nil)
	if err != nil {
		t.Error("Expected error to be nil")
	}

	if seccomp != nil {
		t.Error("Expected seccomp to be nil")
	}
}

func TestSetupSeccompEmpty(t *testing.T) {
	conf := &specs.LinuxSeccomp{}
	seccomp, err := SetupSeccomp(conf)
	if err != nil {
		t.Error("Expected error to be nil")
	}

	if seccomp != nil {
		t.Error("Expected seccomp to be nil")
	}
}

// TestSetupSeccompWrongAction tests that a wrong action triggers an error
func TestSetupSeccompWrongAction(t *testing.T) {
	conf := &specs.LinuxSeccomp{
		DefaultAction: "SCMP_ACT_NON_EXIXTENT_ACTION",
	}
	_, err := SetupSeccomp(conf)
	if err == nil {
		t.Error("Expected error")
	}
}

// TestSetupSeccompWrongArchitecture tests that a wrong architecture triggers an error
func TestSetupSeccompWrongArchitecture(t *testing.T) {
	conf := &specs.LinuxSeccomp{
		DefaultAction: "SCMP_ACT_ALLOW",
		Architectures: []specs.Arch{"SCMP_ARCH_NON_EXISTENT_ARCH"},
	}
	_, err := SetupSeccomp(conf)
	if err == nil {
		t.Error("Expected error")
	}
}

func TestSetupSeccomp(t *testing.T) {
	errnoRet := uint(55)
	conf := &specs.LinuxSeccomp{
		DefaultAction:    "SCMP_ACT_ERRNO",
		Architectures:    []specs.Arch{specs.ArchX86_64, specs.ArchARM},
		ListenerPath:     "/var/run/mysocket",
		ListenerMetadata: "mymetadatastring",
		Syscalls: []specs.LinuxSyscall{
			{
				Names:  []string{"clone"},
				Action: "SCMP_ACT_ALLOW",
				Args: []specs.LinuxSeccompArg{
					{
						Index:    0,
						Value:    unix.CLONE_NEWNS | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWUSER | unix.CLONE_NEWPID | unix.CLONE_NEWNET | unix.CLONE_NEWCGROUP,
						ValueTwo: 0,
						Op:       "SCMP_CMP_MASKED_EQ",
					},
				},
			},
			{
				Names:  []string{"semctl"},
				Action: "SCMP_ACT_KILL",
			},
			{
				Names:  []string{"semget"},
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Names:    []string{"send"},
				Action:   "SCMP_ACT_ERRNO",
				ErrnoRet: &errnoRet,
			},
			{
				Names:  []string{"lchown"},
				Action: "SCMP_ACT_TRAP",
			},
			{
				Names:  []string{"lremovexattr"},
				Action: "SCMP_ACT_TRACE",
			},
			{
				Names:  []string{"mbind"},
				Action: "SCMP_ACT_LOG",
			},
			{
				Names:  []string{"mknod"},
				Action: "SCMP_ACT_NOTIFY",
			},
			{
				Names:  []string{"rmdir"},
				Action: "SCMP_ACT_KILL_THREAD",
			},
			{
				Names:  []string{"mkdir"},
				Action: "SCMP_ACT_KILL_PROCESS",
			},
		},
	}
	seccomp, err := SetupSeccomp(conf)
	if err != nil {
		t.Errorf("Couldn't create Seccomp config: %v", err)
	}

	if seccomp.DefaultAction != configs.Errno {
		t.Error("Wrong conversion for DefaultAction")
	}

	if len(seccomp.Architectures) != 2 {
		t.Error("Wrong number of architectures")
	}

	if seccomp.Architectures[0] != "amd64" || seccomp.Architectures[1] != "arm" {
		t.Error("Expected architectures are not found")
	}

	if seccomp.ListenerPath != "/var/run/mysocket" {
		t.Error("Expected ListenerPath is wrong")
	}

	if seccomp.ListenerMetadata != "mymetadatastring" {
		t.Error("Expected ListenerMetadata is wrong")
	}

	calls := seccomp.Syscalls

	if len(calls) != len(conf.Syscalls) {
		t.Error("Mismatched number of syscalls")
	}

	for _, call := range calls {
		switch call.Name {
		case "clone":
			if call.Action != configs.Allow {
				t.Error("Wrong conversion for the clone syscall action")
			}
			expectedCloneSyscallArgs := configs.Arg{
				Index:    0,
				Op:       configs.MaskEqualTo,
				Value:    unix.CLONE_NEWNS | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWUSER | unix.CLONE_NEWPID | unix.CLONE_NEWNET | unix.CLONE_NEWCGROUP,
				ValueTwo: 0,
			}
			if expectedCloneSyscallArgs != *call.Args[0] {
				t.Errorf("Wrong arguments conversion for the clone syscall under test")
			}
		case "semctl":
			if call.Action != configs.Kill {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
		case "semget":
			if call.Action != configs.Errno {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
			if call.ErrnoRet != nil {
				t.Errorf("Wrong error ret for the %s syscall", call.Name)
			}
		case "send":
			if call.Action != configs.Errno {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
			if *call.ErrnoRet != errnoRet {
				t.Errorf("Wrong error ret for the %s syscall", call.Name)
			}
		case "lchown":
			if call.Action != configs.Trap {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
		case "lremovexattr":
			if call.Action != configs.Trace {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
		case "mbind":
			if call.Action != configs.Log {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
		case "mknod":
			if call.Action != configs.Notify {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
		case "rmdir":
			if call.Action != configs.KillThread {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
		case "mkdir":
			if call.Action != configs.KillProcess {
				t.Errorf("Wrong conversion for the %s syscall action", call.Name)
			}
		default:
			t.Errorf("Unexpected syscall %s found", call.Name)
		}
	}
}

func TestLinuxCgroupWithMemoryResource(t *testing.T) {
	cgroupsPath := "/user/cgroups/path/id"

	spec := &specs.Spec{}
	devices := []specs.LinuxDeviceCgroup{
		{
			Allow:  false,
			Access: "rwm",
		},
	}

	limit := int64(100)
	reservation := int64(50)
	swap := int64(20)
	kernel := int64(40)
	kernelTCP := int64(45)
	swappiness := uint64(1)
	swappinessPtr := &swappiness
	disableOOMKiller := true
	resources := &specs.LinuxResources{
		Devices: devices,
		Memory: &specs.LinuxMemory{
			Limit:            &limit,
			Reservation:      &reservation,
			Swap:             &swap,
			Kernel:           &kernel,
			KernelTCP:        &kernelTCP,
			Swappiness:       swappinessPtr,
			DisableOOMKiller: &disableOOMKiller,
		},
	}
	spec.Linux = &specs.Linux{
		CgroupsPath: cgroupsPath,
		Resources:   resources,
	}

	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: false,
		Spec:             spec,
	}

	cgroup, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	if cgroup.Path != cgroupsPath {
		t.Errorf("Wrong cgroupsPath, expected '%s' got '%s'", cgroupsPath, cgroup.Path)
	}
	if cgroup.Resources.Memory != limit {
		t.Errorf("Expected to have %d as memory limit, got %d", limit, cgroup.Resources.Memory)
	}
	if cgroup.Resources.MemoryReservation != reservation {
		t.Errorf("Expected to have %d as memory reservation, got %d", reservation, cgroup.Resources.MemoryReservation)
	}
	if cgroup.Resources.MemorySwap != swap {
		t.Errorf("Expected to have %d as swap, got %d", swap, cgroup.Resources.MemorySwap)
	}
	if cgroup.Resources.MemorySwappiness != swappinessPtr {
		t.Errorf("Expected to have %d as memory swappiness, got %d", swappinessPtr, cgroup.Resources.MemorySwappiness)
	}
	if cgroup.Resources.OomKillDisable != disableOOMKiller {
		t.Errorf("The OOMKiller should be enabled")
	}
}

func TestLinuxCgroupSystemd(t *testing.T) {
	cgroupsPath := "parent:scopeprefix:name"

	spec := &specs.Spec{}
	spec.Linux = &specs.Linux{
		CgroupsPath: cgroupsPath,
	}

	opts := &CreateOpts{
		UseSystemdCgroup: true,
		Spec:             spec,
	}

	cgroup, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	expectedParent := "parent"
	if cgroup.Parent != expectedParent {
		t.Errorf("Expected to have %s as Parent instead of %s", expectedParent, cgroup.Parent)
	}

	expectedScopePrefix := "scopeprefix"
	if cgroup.ScopePrefix != expectedScopePrefix {
		t.Errorf("Expected to have %s as ScopePrefix instead of %s", expectedScopePrefix, cgroup.ScopePrefix)
	}

	expectedName := "name"
	if cgroup.Name != expectedName {
		t.Errorf("Expected to have %s as Name instead of %s", expectedName, cgroup.Name)
	}
}

func TestLinuxCgroupSystemdWithEmptyPath(t *testing.T) {
	cgroupsPath := ""

	spec := &specs.Spec{}
	spec.Linux = &specs.Linux{
		CgroupsPath: cgroupsPath,
	}

	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: true,
		Spec:             spec,
	}

	cgroup, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	expectedParent := ""
	if cgroup.Parent != expectedParent {
		t.Errorf("Expected to have %s as Parent instead of %s", expectedParent, cgroup.Parent)
	}

	expectedScopePrefix := "runc"
	if cgroup.ScopePrefix != expectedScopePrefix {
		t.Errorf("Expected to have %s as ScopePrefix instead of %s", expectedScopePrefix, cgroup.ScopePrefix)
	}

	if cgroup.Name != opts.CgroupName {
		t.Errorf("Expected to have %s as Name instead of %s", opts.CgroupName, cgroup.Name)
	}
}

func TestLinuxCgroupSystemdWithInvalidPath(t *testing.T) {
	cgroupsPath := "/user/cgroups/path/id"

	spec := &specs.Spec{}
	spec.Linux = &specs.Linux{
		CgroupsPath: cgroupsPath,
	}

	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: true,
		Spec:             spec,
	}

	_, err := CreateCgroupConfig(opts, nil)
	if err == nil {
		t.Error("Expected to produce an error if not using the correct format for cgroup paths belonging to systemd")
	}
}

func TestLinuxCgroupsPathSpecified(t *testing.T) {
	cgroupsPath := "/user/cgroups/path/id"

	spec := &specs.Spec{}
	spec.Linux = &specs.Linux{
		CgroupsPath: cgroupsPath,
	}

	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: false,
		Spec:             spec,
	}

	cgroup, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	if cgroup.Path != cgroupsPath {
		t.Errorf("Wrong cgroupsPath, expected '%s' got '%s'", cgroupsPath, cgroup.Path)
	}
}

func TestLinuxCgroupsPathNotSpecified(t *testing.T) {
	spec := &specs.Spec{}
	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: false,
		Spec:             spec,
	}

	cgroup, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	if cgroup.Path != "" {
		t.Errorf("Wrong cgroupsPath, expected it to be empty string, got '%s'", cgroup.Path)
	}
}

func TestSpecconvExampleValidate(t *testing.T) {
	spec := Example()
	spec.Root.Path = "/"

	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: false,
		Spec:             spec,
	}

	config, err := CreateLibcontainerConfig(opts)
	if err != nil {
		t.Errorf("Couldn't create libcontainer config: %v", err)
	}

	if config.NoNewPrivileges != spec.Process.NoNewPrivileges {
		t.Errorf("specconv NoNewPrivileges mismatch. Expected %v got %v",
			spec.Process.NoNewPrivileges, config.NoNewPrivileges)
	}

	if err := validate.Validate(config); err != nil {
		t.Errorf("Expected specconv to produce valid container config: %v", err)
	}
}

func TestSpecconvNoLinuxSection(t *testing.T) {
	spec := Example()
	spec.Root.Path = "/"
	spec.Linux = nil
	spec.Hostname = ""

	opts := &CreateOpts{
		CgroupName: "ContainerID",
		Spec:       spec,
	}

	config, err := CreateLibcontainerConfig(opts)
	if err != nil {
		t.Errorf("Couldn't create libcontainer config: %v", err)
	}

	if err := validate.Validate(config); err != nil {
		t.Errorf("Expected specconv to produce valid container config: %v", err)
	}
}

func TestDupNamespaces(t *testing.T) {
	spec := &specs.Spec{
		Root: &specs.Root{
			Path: "rootfs",
		},
		Linux: &specs.Linux{
			Namespaces: []specs.LinuxNamespace{
				{
					Type: "pid",
				},
				{
					Type: "pid",
					Path: "/proc/1/ns/pid",
				},
			},
		},
	}

	_, err := CreateLibcontainerConfig(&CreateOpts{
		Spec: spec,
	})

	if !strings.Contains(err.Error(), "malformed spec file: duplicated ns") {
		t.Errorf("Duplicated namespaces should be forbidden")
	}
}

func TestNonZeroEUIDCompatibleSpecconvValidate(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("Test requires userns.")
	}

	spec := Example()
	spec.Root.Path = "/"
	ToRootless(spec)

	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: false,
		Spec:             spec,
		RootlessEUID:     true,
		RootlessCgroups:  true,
	}

	config, err := CreateLibcontainerConfig(opts)
	if err != nil {
		t.Errorf("Couldn't create libcontainer config: %v", err)
	}

	if err := validate.Validate(config); err != nil {
		t.Errorf("Expected specconv to produce valid rootless container config: %v", err)
	}
}

func TestInitSystemdProps(t *testing.T) {
	type inT struct {
		name, value string
	}
	type expT struct {
		isErr bool
		name  string
		value interface{}
	}

	testCases := []struct {
		desc string
		in   inT
		exp  expT
	}{
		{
			in:  inT{"org.systemd.property.TimeoutStopUSec", "uint64 123456789"},
			exp: expT{false, "TimeoutStopUSec", uint64(123456789)},
		},
		{
			desc: "convert USec to Sec (default numeric type)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "456"},
			exp:  expT{false, "TimeoutStopUSec", uint64(456000000)},
		},
		{
			desc: "convert USec to Sec (byte)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "byte 234"},
			exp:  expT{false, "TimeoutStopUSec", uint64(234000000)},
		},
		{
			desc: "convert USec to Sec (int16)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "int16 234"},
			exp:  expT{false, "TimeoutStopUSec", uint64(234000000)},
		},
		{
			desc: "convert USec to Sec (uint16)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "uint16 234"},
			exp:  expT{false, "TimeoutStopUSec", uint64(234000000)},
		},
		{
			desc: "convert USec to Sec (int32)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "int32 234"},
			exp:  expT{false, "TimeoutStopUSec", uint64(234000000)},
		},
		{
			desc: "convert USec to Sec (uint32)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "uint32 234"},
			exp:  expT{false, "TimeoutStopUSec", uint64(234000000)},
		},
		{
			desc: "convert USec to Sec (int64)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "int64 234"},
			exp:  expT{false, "TimeoutStopUSec", uint64(234000000)},
		},
		{
			desc: "convert USec to Sec (uint64)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "uint64 234"},
			exp:  expT{false, "TimeoutStopUSec", uint64(234000000)},
		},
		{
			desc: "convert USec to Sec (float)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "234.789"},
			exp:  expT{false, "TimeoutStopUSec", uint64(234789000)},
		},
		{
			desc: "convert USec to Sec (bool -- invalid value)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "false"},
			exp:  expT{true, "", ""},
		},
		{
			desc: "convert USec to Sec (string -- invalid value)",
			in:   inT{"org.systemd.property.TimeoutStopSec", "'covfefe'"},
			exp:  expT{true, "", ""},
		},
		{
			desc: "convert USec to Sec (bad variable name, no conversion)",
			in:   inT{"org.systemd.property.FOOSec", "123"},
			exp:  expT{false, "FOOSec", 123},
		},
		{
			in:  inT{"org.systemd.property.CollectMode", "'inactive-or-failed'"},
			exp: expT{false, "CollectMode", "inactive-or-failed"},
		},
		{
			desc: "unrelated property",
			in:   inT{"some.other.annotation", "0"},
			exp:  expT{false, "", ""},
		},
		{
			desc: "too short property name",
			in:   inT{"org.systemd.property.Xo", "1"},
			exp:  expT{true, "", ""},
		},
		{
			desc: "invalid character in property name",
			in:   inT{"org.systemd.property.Number1", "1"},
			exp:  expT{true, "", ""},
		},
		{
			desc: "invalid property value",
			in:   inT{"org.systemd.property.ValidName", "invalid-value"},
			exp:  expT{true, "", ""},
		},
	}

	spec := &specs.Spec{}

	for _, tc := range testCases {
		tc := tc
		spec.Annotations = map[string]string{tc.in.name: tc.in.value}

		outMap, err := initSystemdProps(spec)
		// t.Logf("input %+v, expected %+v, got err:%v out:%+v", tc.in, tc.exp, err, outMap)

		if tc.exp.isErr != (err != nil) {
			t.Errorf("input %+v, expecting error: %v, got %v", tc.in, tc.exp.isErr, err)
		}
		expLen := 1 // expect a single item
		if tc.exp.name == "" {
			expLen = 0 // expect nothing
		}
		if len(outMap) != expLen {
			t.Fatalf("input %+v, expected %d, got %d entries: %v", tc.in, expLen, len(outMap), outMap)
		}
		if expLen == 0 {
			continue
		}

		out := outMap[0]
		if tc.exp.name != out.Name {
			t.Errorf("input %+v, expecting name: %q, got %q", tc.in, tc.exp.name, out.Name)
		}
		expValue := dbus.MakeVariant(tc.exp.value).String()
		if expValue != out.Value.String() {
			t.Errorf("input %+v, expecting value: %s, got %s", tc.in, expValue, out.Value)
		}
	}
}

func TestCheckPropertyName(t *testing.T) {
	testCases := []struct {
		in    string
		valid bool
	}{
		{"", false},   // too short
		{"xx", false}, // too short
		{"xxx", true},
		{"someValidName", true},
		{"A name", false},  // space
		{"3335", false},    // numbers
		{"Name1", false},   // numbers
		{"Кир", false},     // non-ascii
		{"მადლობა", false}, // non-ascii
		{"合い言葉", false},    // non-ascii
	}

	for _, tc := range testCases {
		err := checkPropertyName(tc.in)
		if (err == nil) != tc.valid {
			t.Errorf("case %q: expected valid: %v, got error: %v", tc.in, tc.valid, err)
		}
	}
}

func BenchmarkCheckPropertyName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, s := range []string{"", "xx", "xxx", "someValidName", "A name", "Кир", "მადლობა", "合い言葉"} {
			_ = checkPropertyName(s)
		}
	}
}

func TestNullProcess(t *testing.T) {
	spec := Example()
	spec.Process = nil

	_, err := CreateLibcontainerConfig(&CreateOpts{
		Spec: spec,
	})
	if err != nil {
		t.Errorf("Null process should be forbidden")
	}
}

func TestCreateDevices(t *testing.T) {
	spec := Example()

	// dummy uid/gid for /dev/tty; will enable the test to check if createDevices()
	// preferred the spec's device over the redundant default device
	ttyUid := uint32(1000)
	ttyGid := uint32(1000)
	fm := os.FileMode(0o666)

	spec.Linux = &specs.Linux{
		Devices: []specs.LinuxDevice{
			{
				// This is purposely redundant with one of runc's default devices
				Path:     "/dev/tty",
				Type:     "c",
				Major:    5,
				Minor:    0,
				FileMode: &fm,
				UID:      &ttyUid,
				GID:      &ttyGid,
			},
			{
				// This is purposely not redundant with one of runc's default devices
				Path:  "/dev/ram0",
				Type:  "b",
				Major: 1,
				Minor: 0,
			},
		},
	}

	conf := &configs.Config{}

	defaultDevs, err := createDevices(spec, conf)
	if err != nil {
		t.Errorf("failed to create devices: %v", err)
	}

	// Verify the returned default devices has the /dev/tty entry deduplicated
	found := false
	for _, d := range defaultDevs {
		if d.Path == "/dev/tty" {
			if found {
				t.Errorf("createDevices failed: returned a duplicated device entry: %v", defaultDevs)
			}
			found = true
		}
	}

	// Verify that createDevices() placed all default devices in the config
	for _, allowedDev := range AllowedDevices {
		if allowedDev.Path == "" {
			continue
		}

		found := false
		for _, configDev := range conf.Devices {
			if configDev.Path == allowedDev.Path {
				found = true
			}
		}
		if !found {
			configDevPaths := []string{}
			for _, configDev := range conf.Devices {
				configDevPaths = append(configDevPaths, configDev.Path)
			}
			t.Errorf("allowedDevice %s was not found in the config's devices: %v", allowedDev.Path, configDevPaths)
		}
	}

	// Verify that createDevices() deduplicated the /dev/tty entry in the config
	for _, configDev := range conf.Devices {
		if configDev.Path == "/dev/tty" {
			wantDev := &devices.Device{
				Path:     "/dev/tty",
				FileMode: 0o666,
				Uid:      1000,
				Gid:      1000,
				Rule: devices.Rule{
					Type:  devices.CharDevice,
					Major: 5,
					Minor: 0,
				},
			}

			if *configDev != *wantDev {
				t.Errorf("redundant dev was not deduplicated correctly: want %v, got %v", wantDev, configDev)
			}
		}
	}

	// Verify that createDevices() added the entry for /dev/ram0 in the config
	found = false
	for _, configDev := range conf.Devices {
		if configDev.Path == "/dev/ram0" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("device /dev/ram0 not found in config devices; got %v", conf.Devices)
	}
}
