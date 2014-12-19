package namespaces

import (
	"os"
	"os/exec"

	"github.com/docker/libcontainer/configs"
)

type CreateCommand func(container *configs.Config, console, dataPath, init string, childPipe *os.File, args []string) *exec.Cmd
