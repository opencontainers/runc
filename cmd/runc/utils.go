package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	exactArgs = iota
	minArgs
	maxArgs
)

func checkArgs(context *cli.Context, expected, checkType int) error {
	var err error
	cmdName := context.Command.Name
	switch checkType {
	case exactArgs:
		if context.NArg() != expected {
			err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
		}
	case minArgs:
		if context.NArg() < expected {
			err = fmt.Errorf("%s: %q requires a minimum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	case maxArgs:
		if context.NArg() > expected {
			err = fmt.Errorf("%s: %q requires a maximum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	}

	if err != nil {
		fmt.Printf("Incorrect Usage.\n\n")
		_ = cli.ShowCommandHelp(context, cmdName)
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
func setupSpec(context *cli.Context) (*specs.Spec, error) {
	bundle := context.String("bundle")
	if bundle != "" {
		if err := os.Chdir(bundle); err != nil {
			return nil, err
		}
	}
	spec, err := loadSpec(specConfig)
	if err != nil {
		return nil, err
	}
	return spec, nil
}

func revisePidFile(context *cli.Context) error {
	pidFile := context.String("pid-file")
	if pidFile == "" {
		return nil
	}

	// convert pid-file to an absolute path so we can write to the right
	// file after chdir to bundle
	pidFile, err := filepath.Abs(pidFile)
	if err != nil {
		return err
	}
	return context.Set("pid-file", pidFile)
}

// reviseRootDir ensures that the --root option argument,
// if specified, is converted to an absolute and cleaned path,
// and that this path is sane.
func reviseRootDir(context *cli.Context) error {
	if !context.IsSet("root") {
		return nil
	}
	root, err := filepath.Abs(context.GlobalString("root"))
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

	return context.GlobalSet("root", root)
}

// parseBoolOrAuto returns (nil, nil) if s is empty or "auto"
func parseBoolOrAuto(s string) (*bool, error) {
	if s == "" || strings.ToLower(s) == "auto" {
		return nil, nil
	}
	b, err := strconv.ParseBool(s)
	return &b, err
}
