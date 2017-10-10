package validate_test

import (
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/configs/validate"
)

func TestValidate(t *testing.T) {
	for _, s := range []struct {
		config   *configs.Config
		testcase string
		//expectError `true` return error is not nil,`false` is nil.
		expectError bool
	}{
		{
			config: &configs.Config{
				Rootfs: "/var",
			},
			testcase:    "TestValidate",
			expectError: false,
		},
		{
			config: &configs.Config{
				Rootfs:     "/var",
				Namespaces: []configs.Namespace{},
				Networks:   []*configs.Network{{Type: "loopback"}},
			},
			testcase:    "TestValidateNetworkWithoutNETNamespace",
			expectError: true,
		},
		{
			config: &configs.Config{
				Rootfs:     "/var",
				Namespaces: []configs.Namespace{},
				Routes:     []*configs.Route{{Gateway: "255.255.255.0"}},
			},
			testcase:    "TestValidateNetworkRoutesWithoutNETNamespace",
			expectError: true,
		},
		{
			config: &configs.Config{
				Rootfs:   "/var",
				Hostname: "runc",
				Namespaces: configs.Namespaces([]configs.Namespace{
					{Type: configs.NEWUTS},
				},
				),
			},
			testcase:    "TestValidateHostname",
			expectError: false,
		},
		{
			config: &configs.Config{
				Rootfs:   "/var",
				Hostname: "runc",
			},
			testcase:    "TestValidateHostnameWithoutUTSNamespace",
			expectError: true,
		},
		{
			config: &configs.Config{
				Rootfs:    "/var",
				MaskPaths: []string{"/proc/kcore"},
				Namespaces: configs.Namespaces([]configs.Namespace{
					{Type: configs.NEWNS},
				},
				),
			},
			testcase:    "TestValidateSecurityWithMaskPaths",
			expectError: false,
		},
		{
			config: &configs.Config{
				Rootfs:        "/var",
				ReadonlyPaths: []string{"/proc/sys"},
				Namespaces: configs.Namespaces([]configs.Namespace{
					{Type: configs.NEWNS},
				},
				),
			},
			testcase:    "TestValidateSecurityWithROPaths",
			expectError: false,
		},
		{
			config: &configs.Config{
				Rootfs:        "/var",
				MaskPaths:     []string{"/proc/kcore"},
				ReadonlyPaths: []string{"/proc/sys"},
			},
			expectError: true,
		},
		{
			config: &configs.Config{
				Rootfs:      "/var",
				UidMappings: []configs.IDMap{{ContainerID: 123}},
			},
			testcase:    "TestValidateUsernamespaceWithoutUserNS",
			expectError: true,
		},
		{
			config: &configs.Config{
				Rootfs: "/var",
				Sysctl: map[string]string{"net.ctl": "ctl"},
				Namespaces: configs.Namespaces(
					[]configs.Namespace{{
						Type: configs.NEWNET,
						Path: "/proc/self/ns/net",
					},
					},
				),
			},
			testcase:    "TestValidateSysctlWithSameNs",
			expectError: true,
		},
		{
			config: &configs.Config{
				Rootfs:     "/var",
				Sysctl:     map[string]string{"net.ctl": "ctl"},
				Namespaces: []configs.Namespace{},
			},
			testcase:    "TestValidateSysctlWithoutNETNamespace",
			expectError: true,
		},
	} {
		t.Run(s.testcase, testValidateHelper(t, s.config, s.expectError))
	}

}

func testValidateHelper(t *testing.T, config *configs.Config, shouldFail bool) func(t *testing.T) {

	return func(t *testing.T) {
		validator := validate.New()
		err := validator.Validate(config)
		if shouldFail == false {
			if err != nil {
				t.Errorf("expected error to not occur: %+v", err)
			}
		} else {
			if err == nil {
				t.Errorf("expected error to occur but it was nil")
			}
		}
	}
}

func TestValidateWithInvalidRootfs(t *testing.T) {
	dir := "rootfs"
	os.Symlink("/var", dir)
	defer os.Remove(dir)

	config := &configs.Config{
		Rootfs: dir,
	}

	validator := validate.New()
	err := validator.Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateUsernamespace(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("userns is unsupported")
	}
	config := &configs.Config{
		Rootfs: "/var",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUSER},
			},
		),
	}

	validator := validate.New()
	err := validator.Validate(config)
	if err != nil {
		t.Errorf("expected error to not occur %+v", err)
	}
}

func TestValidateSysctl(t *testing.T) {
	sysctl := map[string]string{
		"fs.mqueue.ctl": "ctl",
		"net.ctl":       "ctl",
		"kernel.ctl":    "ctl",
	}

	for k, v := range sysctl {
		config := &configs.Config{
			Rootfs: "/var",
			Sysctl: map[string]string{k: v},
		}

		validator := validate.New()
		err := validator.Validate(config)
		if err == nil {
			t.Error("Expected error to occur but it was nil")
		}
	}
}

func TestValidateValidSysctl(t *testing.T) {
	sysctl := map[string]string{
		"fs.mqueue.ctl": "ctl",
		"net.ctl":       "ctl",
		"kernel.msgmax": "ctl",
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

		validator := validate.New()
		err := validator.Validate(config)
		if err != nil {
			t.Errorf("Expected error to not occur with {%s=%s} but got: %q", k, v, err)
		}
	}
}
