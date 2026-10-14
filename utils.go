package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

const (
	exactArgs = iota
	minArgs
	maxArgs
)

func checkArgs(cmd *cli.Command, expected, checkType int) error {
	var err error
	cmdName := cmd.Name
	switch checkType {
	case exactArgs:
		if cmd.NArg() != expected {
			err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
		}
	case minArgs:
		if cmd.NArg() < expected {
			err = fmt.Errorf("%s: %q requires a minimum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	case maxArgs:
		if cmd.NArg() > expected {
			err = fmt.Errorf("%s: %q requires a maximum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	}

	if err != nil {
		fmt.Printf("Incorrect Usage.\n\n")
		_ = cli.ShowCommandHelp(context.Background(), cmd, cmdName)
		return err
	}
	return nil
}

func logrusToStderr() bool {
	l, ok := logrus.StandardLogger().Out.(*os.File)
	return ok && l.Fd() == os.Stderr.Fd()
}

// fatal prints the error's details if it is a libcontainer specific error type
// then exits the program with an exit status of 1.
func fatal(err error) {
	fatalWithCode(err, 1)
}

func fatalWithCode(err error, ret int) {
	// Make sure the error is written to the logger.
	logrus.Error(err)
	if !logrusToStderr() {
		fmt.Fprintln(os.Stderr, err)
	}

	os.Exit(ret)
}

// setupSpec performs initial setup based on the cli.Context for the container
func setupSpec(cmd *cli.Command) (*specs.Spec, error) {
	bundle := cmd.String("bundle")
	if bundle != "" {
		if err := os.Chdir(bundle); err != nil {
			return nil, err
		}
	}
	config := cmd.String("config")
	if config == "" {
		config = specConfig
	}
	spec, err := loadSpec(config)
	if err != nil {
		return nil, err
	}
	return spec, nil
}

// revisePaths converts relative paths specified by --pid-file,
// --console-socket, --pidfd-socket, and --config to absolute ones,
// so that they are relative to the current directory even after
// chdir to bundle.
func revisePaths(cmd *cli.Command) error {
	for _, name := range []string{"pid-file", "console-socket", "pidfd-socket", "config"} {
		path := cmd.String(name)
		if path == "" {
			continue
		}
		path, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("can't convert --%s argument to absolute path: %w", name, err)
		}
		if err := cmd.Set(name, path); err != nil {
			return err
		}
	}
	return nil
}

// reviseRootDir ensures that the --root option argument,
// if specified, is converted to an absolute and cleaned path,
// and that this path is sane.
func reviseRootDir(cmd *cli.Command) error {
	if !cmd.IsSet("root") {
		return nil
	}
	root, err := filepath.Abs(cmd.String("root"))
	if err != nil {
		return err
	}
	if root == "/" {
		// This can happen if --root argument is
		//  - "" (i.e. empty);
		//  - "." (and the CWD is /);
		//  - "../../.." (enough to get to /);
		//  - "/" (the actual /).
		return errors.New("Option --root argument should not be set to /")
	}

	return cmd.Set("root", root)
}

// parseBoolOrAuto returns (nil, nil) if s is empty or "auto"
func parseBoolOrAuto(s string) (*bool, error) {
	if s == "" || strings.ToLower(s) == "auto" {
		return nil, nil
	}
	b, err := strconv.ParseBool(s)
	return &b, err
}
