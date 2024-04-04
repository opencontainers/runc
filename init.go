package main

import (
	"os"

	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		// Normally, nsexec() never returns, meaning
		// if we are here, it had failed.
		os.Exit(255)
	}
}
