// +build linux

package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: "path to the root of the bundle directory",
		},
		cli.StringFlag{
			Name:  "console",
			Value: "",
			Usage: "specify the pty slave path for use with the container",
		},
	},
	Action: func(context *cli.Context) {
		id := context.Args().First()
		if id == "" {
			fatal(errEmptyID)
		}

		bundle := context.String("bundle")
		if bundle != "" {
			if err := os.Chdir(bundle); err != nil {
				fatal(err)
			}
		}
		spec, err := loadSpec(specConfig)
		if err != nil {
			fatal(err)
		}

		notifySocket := os.Getenv("NOTIFY_SOCKET")
		if notifySocket != "" {
			setupSdNotify(spec, notifySocket)
		}

		if os.Geteuid() != 0 {
			logrus.Fatal("runc should be run as root")
		}
		_, err = createContainer(context, id, spec)
		if err != nil {
			logrus.Fatalf("Container create failed: %v", err)
		}
	},
}
