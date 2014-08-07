package nsinit

import (
	"log"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/namespaces"
	"github.com/docker/libcontainer/syncpipe"
)

var nsenterCommand = cli.Command{
	Name:   "nsenter",
	Usage:  "init process for entering an existing namespace",
	Action: nsenterAction,
}

// this expects that we already have our namespaces setup by the C initializer
// we are expected to finalize the namespace and exec the user's application
func nsenterAction(context *cli.Context) {
	syncPipe, err := syncpipe.NewSyncPipeFromFd(0, 3)
	if err != nil {
		log.Fatalf("unable to create sync pipe: %s", err)
	}

	var config *libcontainer.Config
	if err := syncPipe.ReadFromParent(&config); err != nil {
		log.Fatalf("reading container config from parent: %s", err)
	}

	if err := namespaces.FinalizeSetns(config, context.Args()); err != nil {
		log.Fatalf("failed to nsenter: %s", err)
	}
}
