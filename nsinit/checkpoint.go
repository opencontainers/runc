package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
)

var checkpointCommand = cli.Command{
	Name:  "checkpoint",
	Usage: "checkpoint a running container",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "id", Value: "nsinit", Usage: "specify the ID for a container"},
		cli.StringFlag{Name: "image-path", Value: "", Usage: "path where to save images"},
	},
	Action: func(context *cli.Context) {
		imagePath := context.String("image-path")
		if imagePath == "" {
			fatal(fmt.Errorf("The --image-path option isn't specified"))
		}
		container, err := getContainer(context)
		if err != nil {
			fatal(err)
		}
		// Since a container can be C/R'ed multiple times,
		// the checkpoint directory may already exist.
		if err := os.Mkdir(imagePath, 0655); err != nil && !os.IsExist(err) {
			fatal(err)
		}
		if err := container.Checkpoint(imagePath); err != nil {
			fatal(err)
		}
	},
}
