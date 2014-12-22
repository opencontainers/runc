package main

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

var (
	logPath = os.Getenv("log")
)

func main() {
	app := cli.NewApp()

	app.Name = "nsinit"
	app.Version = "0.1"
	app.Author = "libcontainer maintainers"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "nspid"},
		cli.StringFlag{Name: "console"},
		cli.StringFlag{Name: "root", Value: ".", Usage: "root directory for containers"},
	}

	app.Before = preload

	app.Commands = []cli.Command{
		execCommand,
		initCommand,
		statsCommand,
		configCommand,
		pauseCommand,
		unpauseCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
