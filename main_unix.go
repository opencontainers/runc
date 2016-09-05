// +build linux

package main

import (
	"os"
	"runtime"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/spf13/cobra"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
	}
}

var initCmd = &cobra.Command{
	Short: `initialize the namespaces and launch the process (do not call it outside of runc)`,
	Use:   "init [arguments...]",
	RunE: func(cmd *cobra.Command, args []string) error {
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			// as the error is sent back to the parent there is no need to log
			// or write it to stderr because the parent process will handle this
			os.Exit(1)
		}
		panic("libcontainer: container init failed to exec")
	},
}
