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
		UIDMappings: []configs.IDMap{
			{
				HostID:      1337,
				ContainerID: 0,
				Size:        1,
			},
		},
		GIDMappings: []configs.IDMap{
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
	config.UIDMappings = nil
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur if no uid mappings provided")
	}
}

func TestValidateNonZeroEUIDMappingGid(t *testing.T) {
	config := rootlessEUIDConfig()
	config.GIDMappings = nil
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
	config.UIDMappings[0].Size = 10
	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur when setting uid=2 in mount options and UIDMappings[0].size is 10")
	}

	config.Mounts[0].Data = "uid=20"
	config.UIDMappings[0].Size = 10
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur when setting uid=20 in mount options and UIDMappings[0].size is 10")
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
	config.GIDMappings[0].Size = 10
	if err := Validate(config); err != nil {
		t.Errorf("Expected error to not occur when setting gid=5 in mount options and GIDMappings[0].size is 10")
	}

	config.Mounts[0].Data = "gid=11"
	config.GIDMappings[0].Size = 10
	if err := Validate(config); err == nil {
		t.Errorf("Expected error to occur when setting gid=11 in mount options and GIDMappings[0].size is 10")
	}
}

func BenchmarkRootlessEUIDMount(b *testing.B) {
	config := rootlessEUIDConfig()
	config.GIDMappings[0].Size = 10
	config.Mounts = []*configs.Mount{
		{
			Source:      "devpts",
			Destination: "/dev/pts",
			Device:      "devpts",
			Data:        "newinstance,ptmxmode=0666,mode=0620,uid=0,gid=5",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := rootlessEUIDMount(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}
