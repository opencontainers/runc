package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
)

var execFuncCommand = cli.Command{
	Name:   "func",
	Usage:  "execute a registered function inside an existing container",
	Action: execFuncAction,
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "list", Usage: "list all registered functions"},
		cli.StringFlag{Name: "func", Usage: "function name to exec inside a container"},
	},
}

func execFuncAction(context *cli.Context) {
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

	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// FIXME: remove tty from container config, this should be per process
	config.Tty = false

	state, err := libcontainer.GetState(dataPath)
	if err != nil {
		log.Fatalf("unable to read state.json: %s", err)
	}

	exitCode, err = startInExistingContainer(config, state, context.String("func"), context)
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(exitCode)
}
