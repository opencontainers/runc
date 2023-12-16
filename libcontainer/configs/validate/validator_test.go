package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

func TestValidate(t *testing.T) {
	config := &configs.Config{
		Rootfs: "/var",
	}

	validator := New()
	err := validator.Validate(config)
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

	validator := New()
	err := validator.Validate(config)
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

	validator := New()
	err := validator.Validate(config)
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

	validator := New()
	err := validator.Validate(config)
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

	validator := New()
	err := validator.Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateHostnameWithoutUTSNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs:   "/var",
		Hostname: "runc",
	}

	validator := New()
	err := validator.Validate(config)
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

	validator := New()
	err := validator.Validate(config)
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

	validator := New()
	err := validator.Validate(config)
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

	validator := New()
	err := validator.Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateUserNamespace(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("Test requires userns.")
	}
	config := &configs.Config{
		Rootfs: "/var",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUSER},
			},
		),
		UidMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
		GidMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
	}

	validator := New()
	err := validator.Validate(config)
	if err != nil {
		t.Errorf("expected error to not occur %+v", err)
	}
}

func TestValidateUsernsMappingWithoutNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs:      "/var",
		UidMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
		GidMappings: []configs.IDMap{{HostID: 0, ContainerID: 123, Size: 100}},
	}

	validator := New()
	err := validator.Validate(config)
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

		validator := New()
		err := validator.Validate(config)
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

		validator := New()
		err := validator.Validate(config)
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

	validator := New()
	err := validator.Validate(config)
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

	validator := New()
	if err := validator.Validate(config); err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateSysctlWithoutNETNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs:     "/var",
		Sysctl:     map[string]string{"net.ctl": "ctl"},
		Namespaces: []configs.Namespace{},
	}

	validator := New()
	err := validator.Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateMounts(t *testing.T) {
	testCases := []struct {
		isErr bool
		dest  string
	}{
		// TODO (runc v1.x.x): make these relative paths an error. See https://github.com/opencontainers/runc/pull/3004
		{isErr: false, dest: "not/an/abs/path"},
		{isErr: false, dest: "./rel/path"},
		{isErr: false, dest: "./rel/path"},
		{isErr: false, dest: "../../path"},

		{isErr: false, dest: "/abs/path"},
		{isErr: false, dest: "/abs/but/../unclean"},
	}

	validator := New()

	for _, tc := range testCases {
		config := &configs.Config{
			Rootfs: "/var",
			Mounts: []*configs.Mount{
				{Destination: tc.dest},
			},
		}

		err := validator.Validate(config)
		if tc.isErr && err == nil {
			t.Errorf("mount dest: %s, expected error, got nil", tc.dest)
		}
		if !tc.isErr && err != nil {
			t.Errorf("mount dest: %s, expected nil, got error %v", tc.dest, err)
		}
	}
}
