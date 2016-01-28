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
)

var listCommand = cli.Command{
	Name:  "list",
	Usage: "lists containers started by runc with the given root",
	Action: func(context *cli.Context) {

		// preload the container factory
		if factory == nil {
			err := factoryPreload(context)
			if err != nil {
				logrus.Fatal(err)
				return
			}
		}

		// get the list of containers
		root := context.GlobalString("root")
		absRoot, err := filepath.Abs(root)
		if err != nil {
			logrus.Fatal(err)
			return
		}
		list, err := ioutil.ReadDir(absRoot)

		w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
		fmt.Fprint(w, "ID\tPID\tSTATUS\tCREATED\n")

		// output containers
		for _, item := range list {
			switch {
			case !item.IsDir():
				// do nothing with misc files in the containers directory
			case item.IsDir():
				outputListInfo(item.Name(), w)
			}
		}

		if err := w.Flush(); err != nil {
			logrus.Fatal(err)
		}
	},
}

func outputListInfo(id string, w *tabwriter.Writer) {
	container, err := factory.Load(id)
	if err != nil {
		logrus.Fatal(err)
		return
	}

	containerStatus, err := container.Status()
	if err != nil {
		logrus.Fatal(err)
		return
	}

	state, err := container.State()
	if err != nil {
		logrus.Fatal(err)
		return
	}

	fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", container.ID(),
		state.BaseState.InitProcessPid,
		containerStatus.String(),
		state.BaseState.CreatedTime.Format(time.RFC3339Nano))

}
