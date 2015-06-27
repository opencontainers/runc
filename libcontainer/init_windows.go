package libcontainer

import (
	"github.com/opencontainers/runc/libcontainer/configs"
)

// initConfig is used for transferring parameters from Exec() to Init()
type initConfig struct {
	Args   []string        `json:"args"`
	Env    []string        `json:"env"`
	Cwd    string          `json:"cwd"`
	User   string          `json:"user"`
	Config *configs.Config `json:"config"`
}
