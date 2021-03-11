package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

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
		fmt.Printf("Incorrect Usage.\n\n")
		cli.ShowCommandHelp(context, cmdName)
		return err
	}
	return nil
}

func LogrusToStderr() bool {
	l, ok := logrus.StandardLogger().Out.(*os.File)
	return ok && l.Fd() == os.Stderr.Fd()
}

// Fatal prints the error's details if it is a libcontainer specific error type
// then exits the program with an exit status of 1.
func Fatal(err error) {
	// make sure the error is written to the logger
	logrus.Error(err)
	// If debug is enabled and pkg/errors was used, show its stack trace.
	logrus.Debugf("%+v", err)
	if !LogrusToStderr() {
		fmt.Fprintln(os.Stderr, err)
	}

	os.Exit(1)
}

func RevisePidFile(context *cli.Context) error {
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

// ReviseRootDir convert the root to absolute path
func ReviseRootDir(context *cli.Context) error {
	root := context.GlobalString("root")
	if root == "" {
		return nil
	}

	root, err := filepath.Abs(root)
	if err != nil {
		return err
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
