package main

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "nsinit"
	app.Version = "1"
	app.Author = "libcontainer maintainers"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "nspid"},
		cli.StringFlag{Name: "console"},
		cli.StringFlag{Name: "root", Value: ".", Usage: "root directory for containers"},
	}
	app.Commands = []cli.Command{
		configCommand,
		execCommand,
		initCommand,
		oomCommand,
		pauseCommand,
		statsCommand,
		unpauseCommand,
		stateCommand,
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
