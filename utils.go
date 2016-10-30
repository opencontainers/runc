package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/pflag"
)

// fatal prints the error's details if it is a libcontainer specific error type
// then exits the program with an exit status of 1.
func fatal(err error) {
	// make sure the error is written to the logger
	logrus.Error(err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// setupSpec performs initial setup based on the cli.Context for the container
func setupSpec(flags *pflag.FlagSet) (*specs.Spec, error) {
	if bundle, _ := flags.GetString("bundle"); bundle != "" {
		if err := os.Chdir(bundle); err != nil {
			return nil, err
		}
	}
	spec, err := loadSpec(specConfig)
	if err != nil {
		return nil, err
	}
	notifySocket := os.Getenv("NOTIFY_SOCKET")
	if notifySocket != "" {
		setupSdNotify(spec, notifySocket)
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("runc should be run as root")
	}
	return spec, nil
}
