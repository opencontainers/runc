package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func init() {
	// Support to print stack when receive signal SIGUSR1
	sc := make(chan os.Signal, 1)
	handleSignals(sc)
	signal.Notify(sc, unix.SIGUSR1)
	if len(os.Args) > 1 && os.Args[1] == "init" {
		// This is the golang entry point for runc init, executed
		// before main() but after libcontainer/nsenter's nsexec().
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()

		level, err := strconv.Atoi(os.Getenv("_LIBCONTAINER_LOGLEVEL"))
		if err != nil {
			panic(err)
		}

		logPipeFd, err := strconv.Atoi(os.Getenv("_LIBCONTAINER_LOGPIPE"))
		if err != nil {
			panic(err)
		}

		logrus.SetLevel(logrus.Level(level))
		logrus.SetOutput(os.NewFile(uintptr(logPipeFd), "logpipe"))
		logrus.SetFormatter(new(logrus.JSONFormatter))
		logrus.Debug("child process in init()")

		if err := libcontainer.StartInitialization(); err != nil {
			// as the error is sent back to the parent there is no need to log
			// or write it to stderr because the parent process will handle this
			os.Exit(1)
		}
		panic("libcontainer: container init failed to exec")
	}
}

func handleSignals(signals chan os.Signal) {
	go func() {
		for {
			select {
			case s := <-signals:
				logrus.Debugf("received signal %v ", s)
				switch s {
				case unix.SIGUSR1:
					dumpStacks()
				default:
				}
			}
		}
	}()
}

func dumpStacks() {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	fmt.Fprintln(os.Stderr, string(buf))
}
