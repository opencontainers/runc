package validate

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func rootlessEUIDConfig() *configs.Config {
	return &configs.Config{
		Rootfs:          "/var",
		RootlessEUID:    true,
		RootlessCgroups: true,
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUSER},
			},
		),
		UidMappings: []configs.IDMap{
			{
				HostID:      1337,
				ContainerID: 0,
				Size:        1,
			},
		},
		GidMappings: []configs.IDMap{
			{
				HostID:      7331,
				ContainerID: 0,
				Size:        1,
			},
		},
	}
}

func TestValidateRootlessEUID(t *testing.T) {
	config := rootlessEUIDConfig()
	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

/* rootlessEUIDMappings */

func TestValidateRootlessEUIDUserns(t *testing.T) {
	config := rootlessEUIDConfig()
	config.Namespaces = nil
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur if user namespaces not set")
	}
}

func TestValidateRootlessEUIDMappingUid(t *testing.T) {
	config := rootlessEUIDConfig()
	config.UidMappings = nil
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur if no uid mappings provided")
	}
}

func TestValidateNonZeroEUIDMappingGid(t *testing.T) {
	config := rootlessEUIDConfig()
	config.GidMappings = nil
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur if no gid mappings provided")
	}
}

/* rootlessEUIDMount() */

func TestValidateRootlessEUIDMountUid(t *testing.T) {
	config := rootlessEUIDConfig()
	config.Mounts = []*configs.Mount{
		{
			Source:      "devpts",
			Destination: "/dev/pts",
			Device:      "devpts",
		},
	}

	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur when uid= not set in mount options: %+v", err)
	}

	config.Mounts[0].Data = "uid=5"
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur when setting uid=5 in mount options")
	}

	config.Mounts[0].Data = "uid=0"
	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur when setting uid=0 in mount options: %+v", err)
	}

	config.Mounts[0].Data = "uid=2"
	config.UidMappings[0].Size = 10
	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur when setting uid=2 in mount options and UidMapping[0].size is 10")
	}

	config.Mounts[0].Data = "uid=20"
	config.UidMappings[0].Size = 10
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur when setting uid=20 in mount options and UidMapping[0].size is 10")
	}
}

func TestValidateRootlessEUIDMountGid(t *testing.T) {
	config := rootlessEUIDConfig()
	config.Mounts = []*configs.Mount{
		{
			Source:      "devpts",
			Destination: "/dev/pts",
			Device:      "devpts",
		},
	}

	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur when gid= not set in mount options: %+v", err)
	}

	config.Mounts[0].Data = "gid=5"
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur when setting gid=5 in mount options")
	}

	config.Mounts[0].Data = "gid=0"
	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur when setting gid=0 in mount options: %+v", err)
	}

	config.Mounts[0].Data = "gid=5"
	config.GidMappings[0].Size = 10
	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur when setting gid=5 in mount options and GidMapping[0].size is 10")
	}

	config.Mounts[0].Data = "gid=11"
	config.GidMappings[0].Size = 10
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur when setting gid=11 in mount options and GidMapping[0].size is 10")
	}
}
