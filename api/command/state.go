package command

import (
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
			cs, err := a.State(id)
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
