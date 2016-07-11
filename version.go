package main

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

// version will be populated by the Makefile, read from
// VERSION file of the source code.
var version = ""

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

var versionCommand = cli.Command{
	Name:        "version",
	Usage:       "output the runtime version",
	Description: `The version command outputs the runtime version.`,
	Action:      printVersion,
}

func printVersion(context *cli.Context) (err error) {
	if version == "" {
		_, err = fmt.Print("runc unknown\n")
	} else {
		_, err = fmt.Printf("runc %s\n", version)
	}
	if err != nil {
		return err
	}
	if gitCommit != "" {
		_, err = fmt.Printf("commit: %s\n", gitCommit)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Printf("spec: %s\n", specs.Version)
	return err
}
