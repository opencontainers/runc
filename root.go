package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
)

type runcCommander struct {
	Version string

	// these are for command line parsing - no need to set
	debug               bool
	log                 string
	logFormat           string
	root                string
	criu                string
	enableSystemdCgroup bool
}

func init() {
	logrus.SetOutput(ioutil.Discard)
}

func (r *runcCommander) GetCommand() *cobra.Command {
	var runcCmd = &cobra.Command{
		Short:         usage,
		Use:           "runc [global options] command [command options] [arguments...]",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Println("runc version " + r.Version)
			} else {
				return cmd.Usage()
			}
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if r.debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			if r.log != "" {
				f, err := os.OpenFile(r.log, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0666)
				if err != nil {
					return err
				}
				logrus.SetOutput(f)
			}
			switch r.logFormat {
			case "text":
				// retain logrus's default.
			case "json":
				logrus.SetFormatter(new(logrus.JSONFormatter))
			default:
				return fmt.Errorf("unknown log-format %q", r.logFormat)
			}
			return nil
		},
	}

	// Set Usage&Help template
	runcCmd.SetUsageTemplate(usageTemplate)
	runcCmd.SetHelpTemplate(helpTemplate)

	// Global Options
	runcCmd.PersistentFlags().BoolVar(&r.debug, "debug", false, "enable debug output for logging")
	runcCmd.PersistentFlags().StringVar(&r.log, "log", "/dev/null", "set the log file path where internal debug information is written")
	runcCmd.PersistentFlags().StringVar(&r.logFormat, "log-format", "text", "set the format used by logs ('text' (default), or 'json')")
	runcCmd.PersistentFlags().StringVar(&r.root, "root", "/run/runc", "root directory for storage of container state (this should be located in tmpfs)")
	runcCmd.PersistentFlags().StringVar(&r.criu, "criu", "criu", "path to the criu binary used for checkpoint and restore")
	runcCmd.PersistentFlags().BoolVar(&r.enableSystemdCgroup, "systemd-cgroup", false,
		"enable systemd cgroup support, expects cgroupsPath to be of form \"slice:prefix:name\" for e.g. \"system.slice:runc:434234\"")
	runcCmd.PersistentFlags().BoolP("version", "v", false, "print the version")

	// Sub-commands
	runcCmd.AddCommand(checkpointCmd)
	runcCmd.AddCommand(createCmd)
	runcCmd.AddCommand(deleteCmd)
	runcCmd.AddCommand(eventsCmd)
	runcCmd.AddCommand(execCmd)
	runcCmd.AddCommand(initCmd)
	runcCmd.AddCommand(killCmd)
	runcCmd.AddCommand(listCmd)
	runcCmd.AddCommand(pauseCmd)
	runcCmd.AddCommand(resumeCmd)
	runcCmd.AddCommand(psCmd)
	runcCmd.AddCommand(restoreCmd)
	runcCmd.AddCommand(runCmd)
	runcCmd.AddCommand(specCmd)
	runcCmd.AddCommand(startCmd)
	runcCmd.AddCommand(stateCmd)
	runcCmd.AddCommand(updateCmd)

	return runcCmd
}

var usageTemplate = `NAME:
   {{.CommandPath}} - {{.Short | trim}}

USAGE:
   {{if not .HasSubCommands}}{{.UseLine}}{{end}}{{if .HasSubCommands}}{{.CommandPath}} [global options] command [command options] [arguments...]{{end}}{{if .Long}}

DESCRIPTION:
   {{.Long}}{{end}}{{if .HasExample}}

EXAMPLES:
{{ .Example }}{{end}}{{if not .HasSubCommands}}{{ if .HasAvailableLocalFlags}}

OPTIONS:
{{.LocalFlags.FlagUsages | trimRightSpace}}{{end}}{{end}}{{ if .HasAvailableSubCommands}}

COMMANDS:{{range .Commands}}{{if .IsAvailableCommand}}
     {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{ if .HasSubCommands }}{{if .HasAvailablePersistentFlags}}

GLOBAL OPTIONS:
{{.Flags.FlagUsages | trimRightSpace}}{{end}}{{end}}

`

var helpTemplate = `{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
