package devices

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/sirupsen/logrus"

	"github.com/opencontainers/cgroups"
	devices "github.com/opencontainers/cgroups/devices/config"
)

// systemdProperties takes the configured device rules and generates a
// corresponding set of systemd properties to configure the devices correctly.
func systemdProperties(r *cgroups.Resources, sdVer int) ([]systemdDbus.Property, error) {
	if r.SkipDevices {
		return nil, nil
	}

	properties := []systemdDbus.Property{
		// When we later add DeviceAllow=/dev/foo properties, we are
		// appending devices to the allow list for the unit. However,
		// if this is an existing unit, it already has DeviceAllow=
		// entries, and we need to clear them all before applying the
		// new set. (We also do this for new units, mainly for safety
		// to ensure we only enable the devices we expect.)
		//
		// To clear any existing DeviceAllow= rules, we have to add an
		// empty DeviceAllow= property.
		newProp("DeviceAllow", []deviceAllowEntry{}),
		// Always run in the strictest white-list mode.
		newProp("DevicePolicy", "strict"),
	}

	// Figure out the set of rules.
	configEmu := emulator{}
	for _, rule := range r.Devices {
		if err := configEmu.Apply(*rule); err != nil {
			return nil, fmt.Errorf("unable to apply rule for systemd: %w", err)
		}
	}
	// systemd doesn't support blacklists. So we log a warning, and tell
	// systemd to act as a deny-all whitelist. This ruleset will be replaced
	// with our normal fallback code. This may result in spurious errors, but
	// the only other option is to error out here.
	if configEmu.IsBlacklist() {
		// However, if we're dealing with an allow-all rule then we can do it.
		if configEmu.IsAllowAll() {
			return allowAllDevices(), nil
		}
		logrus.Warn("systemd doesn't support blacklist device rules -- applying temporary deny-all rule")
		return properties, nil
	}

	// Now generate the set of rules we actually need to apply. Unlike the
	// normal devices cgroup, in "strict" mode systemd defaults to a deny-all
	// whitelist which is the default for devices.Emulator.
	finalRules, err := configEmu.Rules()
	if err != nil {
		return nil, fmt.Errorf("unable to get simplified rules for systemd: %w", err)
	}
	var deviceAllowList []deviceAllowEntry
	for _, rule := range finalRules {
		if !rule.Allow {
			// Should never happen.
			return nil, fmt.Errorf("[internal error] cannot add deny rule to systemd DeviceAllow list: %v", *rule)
		}
		switch rule.Type {
		case devices.BlockDevice, devices.CharDevice:
		default:
			// Should never happen.
			return nil, fmt.Errorf("invalid device type for DeviceAllow: %v", rule.Type)
		}

		entry := deviceAllowEntry{
			Perms: string(rule.Permissions),
		}

		// systemd has a fairly odd (though understandable) syntax here, and
		// because of the OCI configuration format we have to do quite a bit of
		// trickery to convert things:
		//
		//  * Concrete rules with non-wildcard major/minor numbers have to use
		//    /dev/{block,char}/MAJOR:minor paths. Before v240, systemd uses
		//    stat(2) on such paths to look up device properties, meaning we
		//    cannot add whitelist rules for devices that don't exist. Since v240,
		//    device properties are parsed from the path string.
		//
		//    However, path globbing is not supported for path-based rules so we
		//    need to handle wildcards in some other manner.
		//
		//  * If systemd older than v240 is used, wildcard-minor rules
		//    have to specify a "device group name" (the second column
		//    in /proc/devices).
		//
		//  * Wildcard (major and minor) rules can just specify a glob with the
		//    type ("char-*" or "block-*").
		//
		// The only type of rule we can't handle is wildcard-major rules, and
		// so we'll give a warning in that case (note that the fallback code
		// will insert any rules systemd couldn't handle). What amazing fun.

		if rule.Major == devices.Wildcard {
			// "_ *:n _" rules aren't supported by systemd.
			if rule.Minor != devices.Wildcard {
				logrus.Warnf("systemd doesn't support '*:n' device rules -- temporarily ignoring rule: %v", *rule)
				continue
			}

			// "_ *:* _" rules just wildcard everything.
			prefix, err := groupPrefix(rule.Type)
			if err != nil {
				return nil, err
			}
			entry.Path = prefix + "*"
		} else if rule.Minor == devices.Wildcard {
			if sdVer >= 240 {
				// systemd v240+ allows for {block,char}-MAJOR syntax.
				prefix, err := groupPrefix(rule.Type)
				if err != nil {
					return nil, err
				}
				entry.Path = prefix + strconv.FormatInt(rule.Major, 10)
			} else {
				// For older systemd, "_ n:* _" rules require a device group from /proc/devices.
				group, err := findDeviceGroup(rule.Type, rule.Major)
				if err != nil {
					return nil, fmt.Errorf("unable to find device '%v/%d': %w", rule.Type, rule.Major, err)
				}
				if group == "" {
					// Couldn't find a group.
					logrus.Warnf("could not find device group for '%v/%d' in /proc/devices -- temporarily ignoring rule: %v", rule.Type, rule.Major, *rule)
					continue
				}
				entry.Path = group
			}
		} else {
			// "_ n:m _" rules are just a path in /dev/{block,char}/.
			switch rule.Type {
			case devices.BlockDevice:
				entry.Path = fmt.Sprintf("/dev/block/%d:%d", rule.Major, rule.Minor)
			case devices.CharDevice:
				entry.Path = fmt.Sprintf("/dev/char/%d:%d", rule.Major, rule.Minor)
			}
			if sdVer < 240 {
				// Old systemd versions use stat(2) on path to find out device major:minor
				// numbers and type. If the path doesn't exist, it will not add the rule,
				// emitting a warning instead.
				// Since all of this logic is best-effort anyway (we manually set these
				// rules separately to systemd) we can safely skip entries that don't
				// have a corresponding path.
				if _, err := os.Stat(entry.Path); err != nil {
					continue
				}
			}
		}
		deviceAllowList = append(deviceAllowList, entry)
	}

	properties = append(properties, newProp("DeviceAllow", deviceAllowList))
	return properties, nil
}

func newProp(name string, units any) systemdDbus.Property {
	return systemdDbus.Property{
		Name:  name,
		Value: dbus.MakeVariant(units),
	}
}

func groupPrefix(ruleType devices.Type) (string, error) {
	switch ruleType {
	case devices.BlockDevice:
		return "block-", nil
	case devices.CharDevice:
		return "char-", nil
	default:
		return "", fmt.Errorf("device type %v has no group prefix", ruleType)
	}
}

// findDeviceGroup tries to find the device group name (as listed in
// /proc/devices) with the type prefixed as required for DeviceAllow, for a
// given (type, major) combination. If more than one device group exists, an
// arbitrary one is chosen.
func findDeviceGroup(ruleType devices.Type, ruleMajor int64) (string, error) {
	fh, err := os.Open("/proc/devices")
	if err != nil {
		return "", err
	}
	defer fh.Close()

	prefix, err := groupPrefix(ruleType)
	if err != nil {
		return "", err
	}
	ruleMajorStr := strconv.FormatInt(ruleMajor, 10) + " "

	scanner := bufio.NewScanner(fh)
	var currentType devices.Type
	for scanner.Scan() {
		// We need to strip spaces because the first number is column-aligned.
		line := strings.TrimSpace(scanner.Text())

		// Handle the "header" lines.
		switch line {
		case "Block devices:":
			currentType = devices.BlockDevice
			continue
		case "Character devices:":
			currentType = devices.CharDevice
			continue
		case "":
			continue
		}

		// Skip lines unrelated to our type.
		if currentType != ruleType {
			continue
		}

		if group, ok := strings.CutPrefix(line, ruleMajorStr); ok {
			return prefix + group, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading /proc/devices: %w", err)
	}
	// Couldn't find the device group.
	return "", nil
}

// DeviceAllow is the dbus type "a(ss)" which means we need a struct
// to represent it in Go.
type deviceAllowEntry struct {
	Path  string
	Perms string
}

func allowAllDevices() []systemdDbus.Property {
	// Setting mode to auto and removing all DeviceAllow rules
	// results in allowing access to all devices.
	return []systemdDbus.Property{
		newProp("DeviceAllow", []deviceAllowEntry{}),
		newProp("DevicePolicy", "auto"),
	}
}
