package main

import (
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"syscall"
	"text/tabwriter"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/configs"
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
		cli.BoolFlag{Name: "list", Usage: "list all registered exec functions"},
		cli.StringFlag{Name: "func", Value: "exec", Usage: "function name to exec inside a container"},
	},
}

func execAction(context *cli.Context) {
	if context.Bool("list") {
		w := tabwriter.NewWriter(os.Stdout, 10, 1, 3, ' ', 0)
		fmt.Fprint(w, "NAME\tUSAGE\n")

		for k, f := range argvs {
			fmt.Fprintf(w, "%s\t%s\n", k, f.Usage)
		}

		w.Flush()

		return
	}

	var exitCode int

	process := &libcontainer.ProcessConfig{
		Args:   context.Args(),
		Env:    context.StringSlice("env"),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	factory, err := libcontainer.New(context.GlobalString("root"), []string{os.Args[0], "init", "--fd", "3", "--"})
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
	if err != nil {
		log.Fatal(err)
	}

	pid, err := container.StartProcess(process)
	if err != nil {
		log.Fatalf("failed to exec: %s", err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("Unable to find the %d process: %s", pid, err)
	}

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
