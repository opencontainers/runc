package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/urfave/cli"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/utils"
)

const (
	usage = `Open Container Initiative contrib/cmd/pidfd-kill

pidfd-kill is an implementation of a consumer of runC's --pidfd-socket API.
After received SIGTERM, pidfd-kill sends the given signal to init process by
pidfd received from --pidfd-socket.

To use pidfd-kill, just specify a socket path at which you want to receive
pidfd:

    $ pidfd-kill [--signal KILL] socket.sock
`
)

func main() {
	app := cli.NewApp()
	app.Name = "pidfd-kill"
	app.Usage = usage

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "signal",
			Value: "SIGKILL",
			Usage: "Signal to send to the init process",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "Path to write the pidfd-kill process ID to",
		},
	}

	app.Action = func(ctx *cli.Context) error {
		args := ctx.Args()
		if len(args) != 1 {
			return errors.New("required a single socket path")
		}

		socketFile := ctx.Args()[0]

		pidFile := ctx.String("pid-file")
		if pidFile != "" {
			pid := fmt.Sprintf("%d\n", os.Getpid())
			if err := os.WriteFile(pidFile, []byte(pid), 0o644); err != nil {
				return err
			}
			defer os.Remove(pidFile)
		}

		sigStr := ctx.String("signal")
		if sigStr == "" {
			sigStr = "SIGKILL"
		}
		sig := unix.SignalNum(sigStr)

		pidfdFile, err := recvPidfd(socketFile)
		if err != nil {
			return err
		}
		defer pidfdFile.Close()

		signalCh := make(chan os.Signal, 16)
		signal.Notify(signalCh, unix.SIGTERM)
		<-signalCh

		return unix.PidfdSendSignal(int(pidfdFile.Fd()), sig, nil, 0)
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "fatal error:", err)
		os.Exit(1)
	}
}

func recvPidfd(socketFile string) (*os.File, error) {
	ln, err := net.Listen("unix", socketFile)
	if err != nil {
		return nil, err
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	unixconn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, errors.New("failed to cast to unixconn")
	}

	socket, err := unixconn.File()
	if err != nil {
		return nil, err
	}
	defer socket.Close()

	return utils.RecvFile(socket)
}
