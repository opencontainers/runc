package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var errEmptyID = errors.New("container id cannot be empty")

// loadFactory returns the configured factory instance for execing containers.
func loadFactory(context *cli.Context) (libcontainer.Factory, error) {
	root := context.GlobalString("root")
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	return libcontainer.New(abs, nil, func(l *libcontainer.FreeBSDFactory) error {
		return nil
	})
}

func getContainer(context *cli.Context) (libcontainer.Container, error) {
	// TODO

	return nil, nil
}

func validateProcessSpec(spec *specs.Process) error {
	// TODO

	return nil
}

func createContainer(context *cli.Context, id string, spec *specs.Spec) (libcontainer.Container, error) {
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		Spec: spec,
	})
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(config.Rootfs); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("rootfs (%q) does not exist", config.Rootfs)
		}
		return nil, err
	}

	factory, err := loadFactory(context)
	if err != nil {
		return nil, err
	}
	return factory.Create(id, config)
}

func startContainer(context *cli.Context, spec *specs.Spec, create bool) (int, error) {
	id := context.Args().First()
	if id == "" {
		return -1, errEmptyID
	}

	_, err := createContainer(context, id, spec)
	if err != nil {
		return -1, err
	}

	return -1, nil
}

func setupSdNotify(spec *specs.Spec, notifySocket string) {
	// TODO
}

func destroy(container libcontainer.Container) {
	if err := container.Destroy(); err != nil {
		logrus.Error(err)
	}
}
