package main

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

var (
	logPath = os.Getenv("log")
	argvs   = make(map[string]*rFunc)
)

func init() {
	argvs["nsenter-exec"] = &rFunc{
		Usage:  "execute a process inside an existing container",
		Action: nsenterExec,
	}

	argvs["nsenter-mknod"] = &rFunc{
		Usage:  "mknod a device inside an existing container",
		Action: nsenterMknod,
	}

	argvs["nsenter-ip"] = &rFunc{
		Usage:  "display the container's network interfaces",
		Action: nsenterIp,
	}
}

func preload(context *cli.Context) error {
	if logPath != "" {
		if err := openLog(logPath); err != nil {
			return err
		}
	}

	return nil
}

func runFunc(f *rFunc) {
	userArgs := findUserArgs()

	config, err := loadConfigFromFd()
	if err != nil {
		log.Fatalf("unable to receive config from sync pipe: %s", err)
	}

	f.Action(config, userArgs)
}

func main() {
	// we need to check our argv 0 for any registred functions to run instead of the
	// normal cli code path
	f, exists := argvs[os.Args[0]]
	if exists {
		runFunc(f)

		return
	}

	app := cli.NewApp()

	app.Name = "nsinit"
	app.Version = "0.1"
	app.Author = "libcontainer maintainers"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "nspid"},
		cli.StringFlag{Name: "console"},
	}

	app.Before = preload

	app.Commands = []cli.Command{
		execCommand,
		initCommand,
		statsCommand,
		configCommand,
		pauseCommand,
		unpauseCommand,
		execFuncCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
