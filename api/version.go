package api

import (
	"fmt"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
)

var (
	// version will be populated by the Makefile, read from VERSION file of the
	// source code.
	version = ""

	// gitCommit will be the hash that the binary was built from and will be
	// populated by the Makefile
	gitCommit = ""
)

// Version specifies all available version strings
type Version struct {
	Runc      string
	Spec      string
	GitCommit string
}

// Version returns the Runc Version instance
func (r *runc) Version() *Version {
	return &Version{
		Runc:      version,
		Spec:      specs.Version,
		GitCommit: gitCommit,
	}
}

// Strings returns a string representation of the Version
func (v *Version) String() string {
	strs := []string{}
	if v.Runc != "" {
		strs = append(strs, v.Runc)
	}
	if v.GitCommit != "" {
		strs = append(strs, fmt.Sprintf("commit: %s", v.GitCommit))
	}
	strs = append(strs, fmt.Sprintf("spec: %s", specs.Version))
	return strings.Join(strs, "\n")
}
