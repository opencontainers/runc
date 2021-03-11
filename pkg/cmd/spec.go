// +build linux

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/urfave/cli"

	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/pkg/constant"
	"github.com/opencontainers/runc/pkg/util"
)

var SpecCommand = cli.Command{
	Name:      "spec",
	Usage:     "create a new specification file",
	ArgsUsage: "",
	Description: `The spec command creates the new specification file named "` + constant.SpecConfig + `" for
the bundle.

The spec generated is just a starter file. Editing of the spec is required to
achieve desired results. For example, the newly generated spec includes an args
parameter that is initially set to call the "sh" command when the container is
started. Calling "sh" may work for an ubuntu container or busybox, but will not
work for containers that do not include the "sh" program.

EXAMPLE:
  To run docker's hello-world container one needs to set the args parameter
in the spec to call hello. This can be done using the sed command or a text
editor. The following commands create a bundle for hello-world, change the
default args parameter in the spec from "sh" to "/hello", then run the hello
command in a new hello-world container named container1:

    mkdir hello
    cd hello
    docker pull hello-world
    docker export $(docker create hello-world) > hello-world.tar
    mkdir rootfs
    tar -C rootfs -xf hello-world.tar
    runc spec
    sed -i 's;"sh";"/hello";' ` + constant.SpecConfig + `
    runc run container1

In the run command above, "container1" is the name for the instance of the
container that you are starting. The name you provide for the container instance
must be unique on your host.

An alternative for generating a customized spec config is to use "oci-runtime-tool", the
sub-command "oci-runtime-tool generate" has lots of options that can be used to do any
customizations as you want, see runtime-tools (https://github.com/opencontainers/runtime-tools)
to get more information.

When starting a container through runc, runc needs root privilege. If not
already running as root, you can use sudo to give runc root privilege. For
example: "sudo runc start container1" will give runc root privilege to start the
container on your host.

Alternatively, you can start a rootless container, which has the ability to run
without root privileges. For this to work, the specification file needs to be
adjusted accordingly. You can pass the parameter --rootless to this command to
generate a proper rootless spec file.

Note that --rootless is not needed when you execute runc as the root in a user namespace
created by an unprivileged user.
`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: "path to the root of the bundle directory",
		},
		cli.BoolFlag{
			Name:  "rootless",
			Usage: "generate a configuration for a rootless container",
		},
	},
	Action: func(context *cli.Context) error {
		if err := util.CheckArgs(context, 0, util.ExactArgs); err != nil {
			return err
		}
		spec := specconv.Example()

		rootless := context.Bool("rootless")
		if rootless {
			specconv.ToRootless(spec)
		}

		checkNoFile := func(name string) error {
			_, err := os.Stat(name)
			if err == nil {
				return fmt.Errorf("File %s exists. Remove it first", name)
			}
			if !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		bundle := context.String("bundle")
		if bundle != "" {
			if err := os.Chdir(bundle); err != nil {
				return err
			}
		}
		if err := checkNoFile(constant.SpecConfig); err != nil {
			return err
		}
		data, err := json.MarshalIndent(spec, "", "\t")
		if err != nil {
			return err
		}
		return ioutil.WriteFile(constant.SpecConfig, data, 0666)
	},
}
