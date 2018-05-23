package command

import (
	"context"

	"github.com/opencontainers/runc/api"
	"github.com/urfave/cli"
)

func NewDeleteCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "delete",
		Usage: "delete any resources held by the container often used with detached container",
		ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.

EXAMPLE:
For example, if the container id is "ubuntu01" and runc list currently shows the
status of "ubuntu01" as "stopped" the following will delete resources held for
"ubuntu01" removing "ubuntu01" from the runc list of containers:

       # runc delete ubuntu01`,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "force, f",
				Usage: "Forcibly deletes the container if it is still running (uses SIGKILL)",
			},
		},
		Action: func(ctx *cli.Context) error {
			if err := CheckArgs(ctx, 1, ExactArgs); err != nil {
				return err
			}
			id, err := GetID(ctx)
			if err != nil {
				return err
			}
			force := ctx.Bool("force")
			a, err := apiNew(NewGlobalConfig(ctx))
			if err != nil {
				return err
			}
			return a.Delete(context.Background(), id, api.DeleteOpts{
				Force: force,
			})
		},
	}
}
