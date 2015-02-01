package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/configs"
	consolepkg "github.com/docker/libcontainer/console"
)

var (
	dataPath  = os.Getenv("data_path")
	console   = os.Getenv("console")
	rawPipeFd = os.Getenv("pipe")
)

var execCommand = cli.Command{
	Name:   "exec",
	Usage:  "execute a new command inside a container",
	Action: execAction,
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "tty", Usage: "allocate a TTY to the container"},
	},
}

func getContainer(context *cli.Context) (libcontainer.Container, error) {
	factory, err := loadFactory(context)
	if err != nil {
		log.Fatal(err)
	}
	id := fmt.Sprintf("%x", md5.Sum([]byte(dataPath)))
	container, err := factory.Load(id)
	if err != nil && !os.IsNotExist(err) {
		var config *configs.Config

		config, err = loadConfig()
		if err != nil {
			log.Fatal(err)
		}
		container, err = factory.Create(id, config)
	}

	return container, err
}

func execAction(context *cli.Context) {
	var (
		master  *os.File
		console string
		err     error

		sigc = make(chan os.Signal, 10)

		stdin  = os.Stdin
		stdout = os.Stdout
		stderr = os.Stderr

		exitCode int
	)

	container, err := getContainer(context)
	if err != nil {
		log.Fatal(err)
	}

	if context.Bool("tty") {
		stdin = nil
		stdout = nil
		stderr = nil

		master, console, err = consolepkg.CreateMasterAndConsole()
		if err != nil {
			log.Fatal(err)
		}

		go io.Copy(master, os.Stdin)
		go io.Copy(os.Stdout, master)

		state, err := term.SetRawTerminal(os.Stdin.Fd())
		if err != nil {
			log.Fatal(err)
		}

		defer term.RestoreTerminal(os.Stdin.Fd(), state)
	}

	process := &libcontainer.ProcessConfig{
		Args:    context.Args(),
		Env:     context.StringSlice("env"),
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Console: console,
	}

	pid, err := container.StartProcess(process)
	if err != nil {
		log.Fatalf("failed to exec: %s", err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("Unable to find the %d process: %s", pid, err)
	}

	go func() {
		resizeTty(master)

		for sig := range sigc {
			switch sig {
			case syscall.SIGWINCH:
				resizeTty(master)
			default:
				p.Signal(sig)
			}
		}
	}()

	ps, err := p.Wait()
	if err != nil {
		log.Fatalf("Unable to wait the %d process: %s", pid, err)
	}
	container.Destroy()

	status := ps.Sys().(syscall.WaitStatus)
	if status.Exited() {
		exitCode = status.ExitStatus()
	} else if status.Signaled() {
		exitCode = -int(status.Signal())
	} else {
		log.Fatalf("Unexpected status")
	}

	os.Exit(exitCode)
}

func resizeTty(master *os.File) {
	if master == nil {
		return
	}

	ws, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return
	}

	if err := term.SetWinsize(master.Fd(), ws); err != nil {
		return
	}
}
