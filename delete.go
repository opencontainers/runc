package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete any resources held by the container often used with detached containers",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.
	 
For example, if the container id is "ubuntu01" and runc list currently shows the
status of "ubuntu01" as "destroyed" the following will delete resources held for
"ubuntu01" removing "ubuntu01" from the runc list of containers:  
	 
       # runc delete ubuntu01`,
	Flags: []cli.Flag{},
	Action: func(context *cli.Context) {
		if os.Geteuid() != 0 {
			logrus.Fatal("runc should be run as root")
		}
		container, err := getContainer(context)
		if err != nil {
			logrus.Fatalf("Container delete failed: %v", err)
			os.Exit(-1)
		}
		deleteContainer(container)
	},
}
