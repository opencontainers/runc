// +build linux

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/opencontainers/runc/libcontainer"
)

const formatOptions = `table, json, yaml, xml, or "go=<go-template text>"`

var listCommand = cli.Command{
	Name:  "list",
	Usage: "lists containers started by runc with the given root",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "format, f",
			Value: "",
			Usage: `select one of: ` + formatOptions + `.

The default format is table.  The following will output the list of containers
in json format:

    # runc list -f json`,
		},
	},
	Action: func(context *cli.Context) {
		format := context.String("format")
		if format != "" {
			switch format {
			default:
				logrus.Fatal("invalid format, valid formats are: " + formatOptions)
			case "":
				format = "table"
			case "json", "yaml", "xml":
			}
		}
		//quiet := context.Bool("quiet")
		//noTrunc := context.Bool("no-trunc")

		factory, err := loadFactory(context)
		if err != nil {
			logrus.Fatal(err)
		}
		// get the list of containers
		root := context.GlobalString("root")
		absRoot, err := filepath.Abs(root)
		if err != nil {
			logrus.Fatal(err)
		}
		list, err := ioutil.ReadDir(absRoot)
		if err != nil {
			logrus.Fatal(err)
		}
		w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
		fmt.Fprint(w, "ID\tPID\tSTATUS\tCREATED\n")
		// output containers
		for _, item := range list {
			if item.IsDir() {
				if err := outputListInfo(item.Name(), factory, w); err != nil {
					logrus.Fatal(err)
				}
			}
		}
		if err := w.Flush(); err != nil {
			logrus.Fatal(err)
		}
	},
}

func outputListInfo(id string, factory libcontainer.Factory, w *tabwriter.Writer) error {
	container, err := factory.Load(id)
	if err != nil {
		return err
	}
	containerStatus, err := container.Status()
	if err != nil {
		return err
	}
	state, err := container.State()
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
		container.ID(),
		state.BaseState.InitProcessPid,
		containerStatus.String(),
		state.BaseState.Created.Format(time.RFC3339Nano))
	return nil
}
