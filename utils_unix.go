package main

import (
	"errors"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli"
)

type CtAct uint8

const (
	CT_ACT_CREATE CtAct = iota + 1
	CT_ACT_RUN
	CT_ACT_RESTORE
)

func getContainer(_ *cli.Context) (*libcontainer.Container, error) {
	return nil, errors.New("unix platform unsupported")
}

func startContainer(_ *cli.Context, _ CtAct, _ *libcontainer.CriuOpts) (int, error) {
	return -1, errors.New("unix platform unsupported")
}