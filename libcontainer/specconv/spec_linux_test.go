// +build linux

package specconv

import (
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs/validate"
	"github.com/opencontainers/runtime-spec/specs-go"
)

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

	cgroup, err := createCgroupConfig(opts)
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
	if cgroup.Resources.KernelMemory != kernel {
		t.Errorf("Expected to have %d as Kernel Memory, got %d", kernel, cgroup.Resources.KernelMemory)
	}
	if cgroup.Resources.KernelMemoryTCP != kernelTCP {
		t.Errorf("Expected to have %d as TCP Kernel Memory, got %d", kernelTCP, cgroup.Resources.KernelMemoryTCP)
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

	cgroup, err := createCgroupConfig(opts)

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

	cgroup, err := createCgroupConfig(opts)

	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	expectedParent := "system.slice"
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

	_, err := createCgroupConfig(opts)
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

	cgroup, err := createCgroupConfig(opts)
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

	cgroup, err := createCgroupConfig(opts)
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

	validator := validate.New()
	if err := validator.Validate(config); err != nil {
		t.Errorf("Expected specconv to produce valid container config: %v", err)
	}
}

func TestDupNamespaces(t *testing.T) {
	spec := &specs.Spec{
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

	if err == nil {
		t.Errorf("Duplicated namespaces should be forbidden")
	}
}

func TestRootlessSpecconvValidate(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("userns is unsupported")
	}

	spec := Example()
	spec.Root.Path = "/"
	ToRootless(spec)

	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: false,
		Spec:             spec,
		Rootless:         true,
	}

	config, err := CreateLibcontainerConfig(opts)
	if err != nil {
		t.Errorf("Couldn't create libcontainer config: %v", err)
	}

	validator := validate.New()
	if err := validator.Validate(config); err != nil {
		t.Errorf("Expected specconv to produce valid rootless container config: %v", err)
	}
}
