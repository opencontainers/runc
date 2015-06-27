package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runc/libcontainer"
)

func destroy(container libcontainer.Container) {
	status, err := container.Status()
	if err != nil {
		logrus.Error(err)
	}
	if status != libcontainer.Checkpointed {
		if err := container.Destroy(); err != nil {
			logrus.Error(err)
		}
	}
}
