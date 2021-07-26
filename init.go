package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/sirupsen/logrus"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		// This is the golang entry point for runc init, executed
		// before main() but after libcontainer/nsenter's nsexec().
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()

		// Configure logrus to talk to the parent.
		level, err := strconv.Atoi(os.Getenv("_LIBCONTAINER_LOGLEVEL"))
		if err != nil {
			panic(fmt.Sprintf("libcontainer: failed to parse _LIBCONTAINER_LOGLEVEL: %s", err))
		}
		logrus.SetLevel(logrus.Level(level))

		logPipeFdStr := os.Getenv("_LIBCONTAINER_LOGPIPE")
		logPipeFd, err := strconv.Atoi(logPipeFdStr)
		if err != nil {
			panic(fmt.Sprintf("libcontainer: failed to convert environment variable _LIBCONTAINER_LOGPIPE=%s to int: %s", logPipeFdStr, err))
		}

		logrus.SetOutput(os.NewFile(uintptr(logPipeFd), "logpipe"))

		logrus.SetFormatter(new(logrus.JSONFormatter))
		logrus.Debug("child process in init()")

		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			// as the error is sent back to the parent there is no need to log
			// or write it to stderr because the parent process will handle this
			os.Exit(1)
		}
		panic("libcontainer: container init failed to exec")
	}
}
