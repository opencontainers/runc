package command

import "github.com/urfave/cli"

func NewStartCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "start",
		Usage: "executes the user defined process in a created container",
		ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
		Description: `The start command executes the user defined process in a created container.`,
		Action: func(context *cli.Context) error {
			if err := CheckArgs(context, 1, ExactArgs); err != nil {
				return err
			}
			id, err := GetID(context)
			if err != nil {
				return err
			}
			a, err := apiNew(NewGlobalConfig(context))
			if err != nil {
				return err
			}
			return a.Start(id)
		},
	}
}
