// Package specconv implements conversion of specifications to libcontainer
// configurations
package specconv

import (
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type CreateOpts struct {
	Spec *specs.Spec
}

// CreateLibcontainerConfig creates a new libcontainer configuration from a
// given specification
func CreateLibcontainerConfig(opts *CreateOpts) (*configs.Config, error) {
	_, err := os.Getwd()

	return nil, err
}
