//go:build linux

package capabilities

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/moby/sys/capability"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
)

func capToStr(c capability.Cap) string {
	return "CAP_" + strings.ToUpper(c.String())
}

var capMap = sync.OnceValues(func() (map[string]capability.Cap, error) {
	list, err := capability.ListSupported()
	if err != nil {
		return nil, err
	}
	cm := make(map[string]capability.Cap, len(list))
	for _, c := range list {
		cm[capToStr(c)] = c
	}
	return cm, nil
})

// KnownCapabilities returns the list of the known capabilities.
// Used by `runc features`.
func KnownCapabilities() []string {
	list := capability.ListKnown()
	res := make([]string, len(list))
	for i, c := range list {
		res[i] = "CAP_" + strings.ToUpper(c.String())
	}
	return res
}

// New creates a new Caps from the given Capabilities config. Unknown Capabilities
// or Capabilities that are unavailable in the current environment are ignored,
// printing a warning instead.
func New(capConfig *configs.Capabilities) (*Caps, error) {
	var c Caps

	_, err := capMap()
	if err != nil {
		return nil, err
	}
	unknownCaps := make(map[string]struct{})
	c.caps = map[capability.CapType][]capability.Cap{
		capability.BOUNDING:    capSlice(capConfig.Bounding, unknownCaps),
		capability.EFFECTIVE:   capSlice(capConfig.Effective, unknownCaps),
		capability.INHERITABLE: capSlice(capConfig.Inheritable, unknownCaps),
		capability.PERMITTED:   capSlice(capConfig.Permitted, unknownCaps),
		capability.AMBIENT:     capSlice(capConfig.Ambient, unknownCaps),
	}
	if c.pid, err = capability.NewPid2(0); err != nil {
		return nil, err
	}
	if len(unknownCaps) > 0 {
		logrus.Warn("ignoring unknown or unavailable capabilities: ", mapKeys(unknownCaps))
	}
	return &c, nil
}

// capSlice converts the slice of capability names in caps, to their numeric
// equivalent, and returns them as a slice. Unknown or unavailable capabilities
// are not returned, but appended to unknownCaps.
func capSlice(caps []string, unknownCaps map[string]struct{}) []capability.Cap {
	cm, _ := capMap()
	out := make([]capability.Cap, 0, len(caps))
	for _, c := range caps {
		if v, ok := cm[c]; !ok {
			unknownCaps[c] = struct{}{}
		} else {
			out = append(out, v)
		}
	}
	return out
}

// mapKeys returns the keys of input in sorted order
func mapKeys(input map[string]struct{}) []string {
	keys := make([]string, 0, len(input))
	for c := range input {
		keys = append(keys, c)
	}
	sort.Strings(keys)
	return keys
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
	c.pid.Clear(capability.CAPS | capability.BOUNDS)
	for _, g := range []capability.CapType{
		capability.EFFECTIVE,
		capability.PERMITTED,
		capability.INHERITABLE,
		capability.BOUNDING,
	} {
		c.pid.Set(g, c.caps[g]...)
	}
	if err := c.pid.Apply(capability.CAPS | capability.BOUNDS); err != nil {
		return fmt.Errorf("can't apply capabilities: %w", err)
	}

	// Old version of capability package used to ignore errors from setting
	// ambient capabilities, which is now fixed (see
	// https://github.com/kolyshkin/capability/pull/3).
	//
	// To maintain backward compatibility, set ambient caps one by one and
	// don't return any errors, only warn.
	ambs := c.caps[capability.AMBIENT]
	err := capability.ResetAmbient()
	if err != nil {
		return fmt.Errorf("can't reset ambient capabilities: %w", err)
	}
	for _, a := range ambs {
		err := capability.SetAmbient(true, a)
		if err != nil {
			logrus.Warnf("can't raise ambient capability %s: %v", capToStr(a), err)
		}
	}

	return nil
}
