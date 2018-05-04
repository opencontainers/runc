package command

import (
	"context"
	"encoding/json"
	"os"

	"github.com/urfave/cli"
)

func NewStateCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "state",
		Usage: "output the state of a container",
		ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container.`,
		Description: `The state command outputs current state information for the
instance of a container.`,
		Action: func(ctx *cli.Context) error {
			if err := CheckArgs(ctx, 1, ExactArgs); err != nil {
				return err
			}
			id, err := GetID(ctx)
			if err != nil {
				return err
			}
			a, err := apiNew(NewGlobalConfig(ctx))
			if err != nil {
				return err
			}
			cs, err := a.State(context.Background(), id)
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(cs, "", "  ")
			if err != nil {
				return err
			}
			os.Stdout.Write(data)
			return nil
		},
	}
}
