package command

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/opencontainers/runc/api"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type Description struct {
	Name        string
	Usage       string
	Version     string
	DefaultRoot string
}

type GlobalConfig struct {
	Debug         bool
	Root          string
	CriuPath      string
	SystemdCgroup bool
}

type APINew func(GlobalConfig) (api.ContainerOperations, error)

// New returns a cli.App for use with a CLI based application
func New(apiNew APINew, desc Description, additionalCommands ...cli.Command) (*cli.App, error) {
	app := cli.NewApp()
	app.Name = desc.Name
	app.Usage = desc.Usage
	app.Version = desc.Version

	// If the command returns an error, cli takes upon itself to print
	// the error on cli.ErrWriter and exit.
	// Use our own writer here to ensure the log gets sent to the right location.
	cli.ErrWriter = &FatalWriter{cli.ErrWriter}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output for logging",
		},
		cli.StringFlag{
			Name:  "log",
			Value: "/dev/null",
			Usage: "set the log file path where internal debug information is written",
		},
		cli.StringFlag{
			Name:  "log-format",
			Value: "text",
			Usage: "set the format used by logs ('text' (default), or 'json')",
		},
		cli.StringFlag{
			Name:  "root",
			Value: desc.DefaultRoot,
			Usage: "root directory for storage of container state (this should be located in tmpfs)",
		},
		cli.StringFlag{
			Name:  "criu",
			Value: "criu",
			Usage: "path to the criu binary used for checkpoint and restore",
		},
		cli.BoolFlag{
			Name:  "systemd-cgroup",
			Usage: "enable systemd cgroup support, expects cgroupsPath to be of form \"slice:prefix:name\" for e.g. \"system.slice:runc:434234\"",
		},
	}
	app.Before = func(context *cli.Context) error {
		if context.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		if path := context.GlobalString("log"); path != "" {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0666)
			if err != nil {
				return err
			}
			logrus.SetOutput(f)
		}
		switch context.GlobalString("log-format") {
		case "text":
			// retain logrus's default.
		case "json":
			logrus.SetFormatter(new(logrus.JSONFormatter))
		default:
			return fmt.Errorf("unknown log-format %q", context.GlobalString("log-format"))
		}
		return nil
	}
	// add standard and additional commands
	app.Commands = []cli.Command{
		NewCheckpointCommand(apiNew),
		NewCreateCommand(apiNew),
		NewDeleteCommand(apiNew),
		NewKillCommand(apiNew),
		NewListCommand(apiNew),
		NewPSCommand(apiNew),
		NewPauseCommand(apiNew),
		NewResumeCommand(apiNew),
		NewRestoreCommand(apiNew),
		NewRunCommand(apiNew),
		NewStartCommand(apiNew),
		NewStateCommand(apiNew),
		NewExecCommand(apiNew),
	}
	app.Commands = append(app.Commands, additionalCommands...)
	return app, nil
}

func NewGlobalConfig(context *cli.Context) GlobalConfig {
	return GlobalConfig{
		Root:          context.GlobalString("root"),
		Debug:         context.GlobalBool("debug"),
		CriuPath:      context.GlobalString("criu"),
		SystemdCgroup: context.GlobalBool("systemd-cgroup"),
	}
}

type FatalWriter struct {
	cliErrWriter io.Writer
}

func (f *FatalWriter) Write(p []byte) (n int, err error) {
	logrus.Error(string(p))
	return f.cliErrWriter.Write(p)
}

const (
	ExactArgs = iota
	MinArgs
	MaxArgs
)

func CheckArgs(context *cli.Context, expected, checkType int) error {
	var err error
	cmdName := context.Command.Name
	switch checkType {
	case ExactArgs:
		if context.NArg() != expected {
			err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
		}
	case MinArgs:
		if context.NArg() < expected {
			err = fmt.Errorf("%s: %q requires a minimum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	case MaxArgs:
		if context.NArg() > expected {
			err = fmt.Errorf("%s: %q requires a maximum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	}
	if err != nil {
		fmt.Printf("incorrect usage.\n\n")
		cli.ShowCommandHelp(context, cmdName)
		return err
	}
	return nil
}

func GetID(context *cli.Context) (string, error) {
	id := context.Args().First()
	if id == "" {
		return "", api.ErrEmptyID
	}
	return id, nil
}

// setupSpec performs initial setup based on the cli.Context for the container
func setupSpec(context *cli.Context) (*specs.Spec, error) {
	bundle := context.String("bundle")
	if bundle != "" {
		if err := os.Chdir(bundle); err != nil {
			return nil, err
		}
	}
	spec, err := LoadSpec("config.json")
	if err != nil {
		return nil, err
	}
	return spec, nil
}

func revisePidFile(context *cli.Context) (string, error) {
	pidFile := context.String("pid-file")
	if pidFile == "" {
		return "", nil
	}
	// convert pid-file to an absolute path so we can write to the right
	// file after chdir to bundle
	return filepath.Abs(pidFile)
}

// LoadSpec loads the specification from the provided path.
func LoadSpec(cPath string) (spec *specs.Spec, err error) {
	cf, err := os.Open(cPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSON specification file %s not found", cPath)
		}
		return nil, err
	}
	defer cf.Close()
	if err = json.NewDecoder(cf).Decode(&spec); err != nil {
		return nil, err
	}
	return spec, validateProcessSpec(spec.Process)
}

func validateProcessSpec(spec *specs.Process) error {
	if spec.Cwd == "" {
		return fmt.Errorf("cwd property must not be empty")
	}
	if !filepath.IsAbs(spec.Cwd) {
		return fmt.Errorf("cwd must be an absolute path")
	}
	if len(spec.Args) == 0 {
		return fmt.Errorf("args must not be empty")
	}
	return nil
}
