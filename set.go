package main

import (
	"github.com/codegangsta/cli"
	"os"
)

var setCommand = cli.Command{
	Name:  "set",
	Usage: "set cgroup resources",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: "path to the root of the bundle directory",
		},
		cli.StringFlag{
			Name:  "config-file, c",
			Value: "config.json",
			Usage: "path to spec file for writing",
		},
		cli.StringFlag{
			Name:  "runtime-file, r",
			Value: "runtime.json",
			Usage: "path for runtime file for writing",
		},
	},
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(err)
		}
		bundle := context.String("bundle")
		if bundle != "" {
			if err := os.Chdir(bundle); err != nil {
				fatal(err)
			}
		}
		spec, rspec, err := loadSpec(context.String("config-file"), context.String("runtime-file"))
		if err != nil {
			fatal(err)
		}
		config, err := createLibcontainerConfig(context.GlobalString("id"), spec, rspec)
		if err != nil {
			fatal(err)
		}
		if err := container.Set(*config); err != nil {
			fatal(err)
		}
	},
}
