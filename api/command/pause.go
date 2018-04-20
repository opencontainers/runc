package command

import "github.com/urfave/cli"

func NewPauseCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "pause",
		Usage: "pause suspends all processes inside the container",
		ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
paused. `,
		Description: `The pause command suspends all processes in the instance of the container.

Use runc list to identiy instances of containers and their current status.`,
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
			return a.Pause(id)
		},
	}
}

func NewResumeCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "resume",
		Usage: "resumes all processes that have been previously paused",
		ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
resumed.`,
		Description: `The resume command resumes all processes in the instance of the container.

Use runc list to identiy instances of containers and their current status.`,
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
			return a.Resume(id)
		},
	}
}
