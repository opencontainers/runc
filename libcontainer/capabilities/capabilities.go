//go:build linux

package capabilities

import (
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
)

const allCapabilityTypes = capability.CAPS | capability.BOUNDING | capability.AMBIENT

var (
	capTypes = []capability.CapType{
		capability.BOUNDING,
		capability.PERMITTED,
		capability.INHERITABLE,
		capability.EFFECTIVE,
		capability.AMBIENT,
	}

	capMap = sync.OnceValue(func() map[string]capability.Cap {
		cm := make(map[string]capability.Cap, capability.CAP_LAST_CAP+1)
		for _, c := range capability.List() {
			if c > capability.CAP_LAST_CAP {
				continue
			}
			cm["CAP_"+strings.ToUpper(c.String())] = c
		}
		return cm
	})
)

// KnownCapabilities returns the list of the known capabilities.
// Used by `runc features`.
func KnownCapabilities() []string {
	list := capability.List()
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
	var (
		err error
		c   Caps
	)

	cm := capMap()
	unknownCaps := make(map[string]struct{})
	ignoredCaps := make(map[string]struct{})
	// capSlice converts the slice of capability names in caps, to their numeric
	// equivalent, and returns them as a slice. Unknown or unavailable capabilities
	// are not returned, but appended to unknownCaps.
	//
	// If mustHave argument is not nil, caps that are not present in mustHave
	// are appended to ignoredCaps instead of the resulting slice.
	capSlice := func(caps []string, mustHave []capability.Cap) []capability.Cap {
		out := make([]capability.Cap, 0, len(caps))
		for _, c := range caps {
			if v, ok := cm[c]; !ok {
				unknownCaps[c] = struct{}{}
			} else {
				if mustHave != nil && !slices.Contains(mustHave, v) {
					ignoredCaps[c] = struct{}{}
					continue
				}
				out = append(out, v)
			}
		}
		return out
	}
	inheritable := capSlice(capConfig.Inheritable, nil)
	// Ambient vector can not contain values not raised in the Inheritable vector,
	// and errors setting ambient capabilities were previously ignored due to a bug
	// (see https://github.com/kolyshkin/capability/pull/3), so to maintain backward
	// compatibility we have to ignore those Ambient caps that are not also raised
	// in Inheritable (and issue a warning).
	ambient := capSlice(capConfig.Ambient, inheritable)
	if len(ignoredCaps) > 0 {
		logrus.Warn("unable to set Ambient capabilities which are not set in Inheritable; ignoring following Ambient capabilities: ", mapKeys(ignoredCaps))
		clear(ignoredCaps)
	}

	c.caps = map[capability.CapType][]capability.Cap{
		capability.BOUNDING:    capSlice(capConfig.Bounding, nil),
		capability.EFFECTIVE:   capSlice(capConfig.Effective, nil),
		capability.INHERITABLE: inheritable,
		capability.PERMITTED:   capSlice(capConfig.Permitted, nil),
		capability.AMBIENT:     ambient,
	}

	if c.pid, err = capability.NewPid2(0); err != nil {
		return nil, err
	}
	if err = c.pid.Load(); err != nil {
		return nil, err
	}
	if len(unknownCaps) > 0 {
		logrus.Warn("ignoring unknown or unavailable capabilities: ", mapKeys(unknownCaps))
	}
	return &c, nil
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
	c.pid.Clear(allCapabilityTypes)
	for _, g := range capTypes {
		c.pid.Set(g, c.caps[g]...)
	}
	return c.pid.Apply(allCapabilityTypes)
}
