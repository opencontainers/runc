// +build linux

package capabilities

import (
	"fmt"
	"strings"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/syndtr/gocapability/capability"
)

const allCapabilityTypes = capability.CAPS | capability.BOUNDING | capability.AMBIENT

var (
	capabilityMap map[string]capability.Cap
	capTypes      = []capability.CapType{
		capability.BOUNDING,
		capability.PERMITTED,
		capability.INHERITABLE,
		capability.EFFECTIVE,
		capability.AMBIENT,
	}
)

func init() {
	capabilityMap = make(map[string]capability.Cap, capability.CAP_LAST_CAP+1)
	for _, c := range capability.List() {
		if c > capability.CAP_LAST_CAP {
			continue
		}
		capabilityMap["CAP_"+strings.ToUpper(c.String())] = c
	}
}

// New creates a new Caps from the given Capabilities config. Unknown Capabilities
// or Capabilities that are unavailable in the current environment produce an error.
func New(capConfig *configs.Capabilities) (*Caps, error) {
	var (
		err error
		c   = Caps{caps: make(map[capability.CapType][]capability.Cap, len(capTypes))}
	)

	if c.caps[capability.BOUNDING], err = capSlice(capConfig.Bounding); err != nil {
		return nil, err
	}
	if c.caps[capability.EFFECTIVE], err = capSlice(capConfig.Effective); err != nil {
		return nil, err
	}
	if c.caps[capability.INHERITABLE], err = capSlice(capConfig.Inheritable); err != nil {
		return nil, err
	}
	if c.caps[capability.PERMITTED], err = capSlice(capConfig.Permitted); err != nil {
		return nil, err
	}
	if c.caps[capability.AMBIENT], err = capSlice(capConfig.Ambient); err != nil {
		return nil, err
	}
	if c.pid, err = capability.NewPid2(0); err != nil {
		return nil, err
	}
	if err = c.pid.Load(); err != nil {
		return nil, err
	}
	return &c, nil
}

// capSlice converts the slice of capability names in caps, to their numeric
// equivalent, and returns them as a slice. Unknown or unavailable capabilities
// produce an error.
func capSlice(caps []string) ([]capability.Cap, error) {
	out := make([]capability.Cap, len(caps))
	for i, c := range caps {
		v, ok := capabilityMap[c]
		if !ok {
			return nil, fmt.Errorf("unknown capability %q", c)
		}
		out[i] = v
	}
	return out, nil
}

// Caps holds the capabilities for a container.
type Caps struct {
	pid  capability.Capabilities
	caps map[capability.CapType][]capability.Cap
}

// ApplyBoundingSet sets the capability bounding set to those specified in the whitelist.
func (c *Caps) ApplyBoundingSet() error {
	c.pid.Clear(capability.BOUNDING)
	c.pid.Set(capability.BOUNDING, c.caps[capability.BOUNDING]...)
	return c.pid.Apply(capability.BOUNDING)
}

// Apply sets all the capabilities for the current process in the config.
func (c *Caps) ApplyCaps() error {
	c.pid.Clear(allCapabilityTypes)
	for _, g := range capTypes {
		c.pid.Set(g, c.caps[g]...)
	}
	return c.pid.Apply(allCapabilityTypes)
}
