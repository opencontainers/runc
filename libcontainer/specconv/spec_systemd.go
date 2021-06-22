// +build linux,!no_systemd

package specconv

import (
	"errors"
	"fmt"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
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

func initSystemdProps(spec *specs.Spec) ([]systemdDbus.Property, error) {
	const keyPrefix = "org.systemd.property."
	var sp []systemdDbus.Property

	for k, v := range spec.Annotations {
		name := strings.TrimPrefix(k, keyPrefix)
		if len(name) == len(k) { // prefix not there
			continue
		}
		if !isValidName(name) {
			return nil, fmt.Errorf("Annotation %s name incorrect: %s", k, name)
		}
		value, err := dbus.ParseVariant(v, dbus.Signature{})
		if err != nil {
			return nil, fmt.Errorf("Annotation %s=%s value parse error: %v", k, v, err)
		}
		if isSecSuffix(name) {
			name = strings.TrimSuffix(name, "Sec") + "USec"
			value, err = convertSecToUSec(value)
			if err != nil {
				return nil, fmt.Errorf("Annotation %s=%s value parse error: %v", k, v, err)
			}
		}
		sp = append(sp, systemdDbus.Property{Name: name, Value: value})
	}

	return sp, nil
}

func initSystemdCgroups(opts *CreateOpts, spec *specs.Spec, c *configs.Cgroup) error {
	var myCgroupPath string

	if !opts.UseSystemdCgroup {
		return initPlainCgroups(opts, spec, c)
	}

	sp, err := initSystemdProps(spec)
	if err != nil {
		return err
	}
	c.Sd.SystemdProps = sp
	if spec.Linux != nil && spec.Linux.CgroupsPath != "" {
		myCgroupPath = spec.Linux.CgroupsPath
	}

	if myCgroupPath == "" {
		// Default for c.Parent is set by systemd cgroup drivers.
		c.ScopePrefix = "runc"
		c.Name = opts.CgroupName
	} else {
		// Parse the path from expected "slice:prefix:name"
		// for e.g. "system.slice:docker:1234"
		parts := strings.Split(myCgroupPath, ":")
		if len(parts) != 3 {
			return fmt.Errorf("expected cgroupsPath to be of format \"slice:prefix:name\" for systemd cgroups, got %q instead", myCgroupPath)
		}
		c.Parent = parts[0]
		c.ScopePrefix = parts[1]
		c.Name = parts[2]
	}

	return nil
}
