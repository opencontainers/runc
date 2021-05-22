// +build gofuzz

package specconv

import (
	"io/ioutil"
	"os"

	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/configs/validate"
	"github.com/opencontainers/runtime-spec/specs-go"

	gofuzzheaders "github.com/AdaLogics/go-fuzz-headers"
)

func newTestRoot(name string) (string, error) {
	dir, err := ioutil.TempDir("", name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func Fuzz(data []byte) int {
	if len(data) < 30 {
		return -1
	}
	f := gofuzzheaders.NewConsumer(data)
	linuxSpec := new(specs.Linux)
	err := f.GenerateStruct(linuxSpec)
	if err != nil {
		return 0
	}

	// Create spec.Spec
	spec := new(specs.Spec)
	err = f.GenerateStruct(spec)
	if err != nil {
		return 0
	}
	spec.Linux = linuxSpec

	// Create CreateOpts
	opts := new(CreateOpts)
	err = f.GenerateStruct(opts)
	if err != nil {
		return 0
	}
	opts.Spec = spec
	rootfs, err := newTestRoot("libcontainer")
	if err != nil {
		return 0
	}
	config := newTemplateConfig(&fuzzTParam{
		rootfs: rootfs,
		userns: false,
	})
	err = f.GenerateStruct(config)
	if err != nil {
		return 0
	}
	config.Rootfs = rootfs

	// Add network
	cn := new(configs.Network)
	err = f.GenerateStruct(cn)
	if err != nil {
		return 0
	}

	config.Networks = []*configs.Network{cn}

	validator := validate.New()
	err = validator.Validate(config)
	if err != nil {
		return 0
	}
	c, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		return 0
	}

	path, err := newTestRoot("fuzzDir")
	if err != nil {
		return 0
	}
	um := systemd.NewUnifiedManager(c, path, false)
	err = um.Set(config)
	err = um.Apply(int(data[0]))
	err = um.Destroy()
	return 1
}
