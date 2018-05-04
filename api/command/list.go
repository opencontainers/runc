package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli"
)

const formatOptions = `table or json`

func NewListCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "list",
		Usage: "lists containers started with the given root",
		ArgsUsage: `

Where the given root is specified via the global option "--root"
(default: "/run/runc").

EXAMPLE 1:
To list containers created via the default "--root":
       # runc list

EXAMPLE 2:
To list containers created using a non-default value for "--root":
       # runc --root value list`,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "format, f",
				Value: "table",
				Usage: `select one of: ` + formatOptions,
			},
			cli.BoolFlag{
				Name:  "quiet, q",
				Usage: "display only container IDs",
			},
		},
		Action: func(ctx *cli.Context) error {
			if err := CheckArgs(ctx, 0, ExactArgs); err != nil {
				return err
			}
			a, err := apiNew(NewGlobalConfig(ctx))
			if err != nil {
				return err
			}
			s, err := a.List(context.Background())
			if err != nil {
				return err
			}
			if ctx.Bool("quiet") {
				for _, item := range s {
					fmt.Println(item.ID)
				}
				return nil
			}
			switch ctx.String("format") {
			case "table":
				w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
				fmt.Fprint(w, "ID\tPID\tSTATUS\tBUNDLE\tCREATED\tOWNER\n")
				for _, item := range s {
					fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n",
						item.ID,
						item.InitProcessPid,
						item.Status,
						item.Bundle,
						item.Created.Format(time.RFC3339Nano),
						item.Owner)
				}
				if err := w.Flush(); err != nil {
					return err
				}
			case "json":
				if err := json.NewEncoder(os.Stdout).Encode(s); err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid format option")
			}
			return nil
		},
	}
}
