package main

import (
	"fmt"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/logs"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
	"runtime"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()

		err := logs.ConfigureLogging(&logs.LoggingConfiguration{
			LogPipeFd: os.Getenv("_LIBCONTAINER_LOGPIPE"),
			LogFormat: "json",
			IsDebug:   true,
		})
		if err != nil {
			panic(fmt.Sprintf("libcontainer: failed to configure logging: %v", err))
		}
		logrus.Debug("child process in init()")
	}
}

var initCommand = cli.Command{
	Name:  "init",
	Usage: `initialize the namespaces and launch the process (do not call it outside of runc)`,
	Action: func(context *cli.Context) error {
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			// as the error is sent back to the parent there is no need to log
			// or write it to stderr because the parent process will handle this
			os.Exit(1)
		}
		panic("libcontainer: container init failed to exec")
	},
}
