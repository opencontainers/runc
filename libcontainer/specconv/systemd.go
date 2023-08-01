//go:build linux && !runc_nosd

package specconv

import (
	"errors"
	"fmt"
	"strings"

	dbus "github.com/godbus/dbus/v5"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Some systemd properties are documented as having "Sec" suffix
// (e.g. TimeoutStopSec) but are expected to have "USec" suffix
// here, so let's provide conversion to improve compatibility.
func convertSecToUSec(value dbus.Variant) (dbus.Variant, error) {
	var sec uint64
	const M = 1000000
	vi := value.Value()
	switch value.Signature().String() {
	case "y":
		sec = uint64(vi.(byte)) * M
	case "n":
		sec = uint64(vi.(int16)) * M
	case "q":
		sec = uint64(vi.(uint16)) * M
	case "i":
		sec = uint64(vi.(int32)) * M
	case "u":
		sec = uint64(vi.(uint32)) * M
	case "x":
		sec = uint64(vi.(int64)) * M
	case "t":
		sec = vi.(uint64) * M
	case "d":
		sec = uint64(vi.(float64) * M)
	default:
		return value, errors.New("not a number")
	}
	return dbus.MakeVariant(sec), nil
}

func initSystemdProps(spec *specs.Spec) (configs.SdProperties, error) {
	const keyPrefix = "org.systemd.property."
	var sp configs.SdProperties

	for k, v := range spec.Annotations {
		name := strings.TrimPrefix(k, keyPrefix)
		if len(name) == len(k) { // prefix not there
			continue
		}
		if err := checkPropertyName(name); err != nil {
			return nil, fmt.Errorf("annotation %s name incorrect: %w", k, err)
		}
		value, err := dbus.ParseVariant(v, dbus.Signature{})
		if err != nil {
			return nil, fmt.Errorf("annotation %s=%s value parse error: %w", k, v, err)
		}
		// Check for Sec suffix.
		if trimName := strings.TrimSuffix(name, "Sec"); len(trimName) < len(name) {
			// Check for a lowercase ascii a-z just before Sec.
			if ch := trimName[len(trimName)-1]; ch >= 'a' && ch <= 'z' {
				// Convert from Sec to USec.
				name = trimName + "USec"
				value, err = convertSecToUSec(value)
				if err != nil {
					return nil, fmt.Errorf("annotation %s=%s value parse error: %w", k, v, err)
				}
			}
		}
		sp = append(sp, configs.SdProperty{Name: name, Value: value})
	}

	return sp, nil
}

func createCgroupConfigSystemd(opts *CreateOpts, c *configs.Cgroup) error {
	spec := opts.Spec

	sp, err := initSystemdProps(spec)
	if err != nil {
		return err
	}
	c.SystemdProps = sp

	if spec.Linux == nil || spec.Linux.CgroupsPath == "" {
		// Default for c.Parent is set by systemd cgroup drivers.
		c.ScopePrefix = "runc"
		c.Name = opts.CgroupName
	} else {
		// Parse the path from expected "slice:prefix:name"
		// for e.g. "system.slice:docker:1234"
		parts := strings.Split(spec.Linux.CgroupsPath, ":")
		if len(parts) != 3 {
			return fmt.Errorf("expected cgroupsPath to be of format \"slice:prefix:name\" for systemd cgroups, got %q instead", spec.Linux.CgroupsPath)
		}
		c.Parent = parts[0]
		c.ScopePrefix = parts[1]
		c.Name = parts[2]
	}

	return nil
}
