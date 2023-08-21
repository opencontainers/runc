package main

import (
	"encoding/json"
	"fmt"

	"github.com/opencontainers/runc/libcontainer/capabilities"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/seccomp"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/types/features"
	"github.com/urfave/cli"
)

var featuresCommand = cli.Command{
	Name:      "features",
	Usage:     "show the enabled features",
	ArgsUsage: "",
	Description: `Show the enabled features.
   The result is parsable as a JSON.
   See https://pkg.go.dev/github.com/opencontainers/runc/types/features for the type definition.
   The types are experimental and subject to change.
`,
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 0, exactArgs); err != nil {
			return err
		}

		tru := true

		feat := features.Features{
			OCIVersionMin: "1.0.0",
			// We usually use specs.Version here, but the runtime-spec version we are vendoring
			// has a bug regarding the use of semver. To workaround it, we just hardcode
			// this in for the 1.1 branch, as we don't expect this to change.
			// See: https://github.com/opencontainers/runtime-spec/issues/1220
			OCIVersionMax: "1.0.3-dev",
			Annotations: map[string]string{
				features.AnnotationRuncVersion:           version,
				features.AnnotationRuncCommit:            gitCommit,
				features.AnnotationRuncCheckpointEnabled: "true",
			},
			Hooks:        configs.KnownHookNames(),
			MountOptions: specconv.KnownMountOptions(),
			Linux: &features.Linux{
				Namespaces:   specconv.KnownNamespaces(),
				Capabilities: capabilities.KnownCapabilities(),
				Cgroup: &features.Cgroup{
					V1:          &tru,
					V2:          &tru,
					Systemd:     &tru,
					SystemdUser: &tru,
				},
				Apparmor: &features.Apparmor{
					Enabled: &tru,
				},
				Selinux: &features.Selinux{
					Enabled: &tru,
				},
			},
		}

		if seccomp.Enabled {
			feat.Linux.Seccomp = &features.Seccomp{
				Enabled:   &tru,
				Actions:   seccomp.KnownActions(),
				Operators: seccomp.KnownOperators(),
				Archs:     seccomp.KnownArchs(),
			}
			major, minor, patch := seccomp.Version()
			feat.Annotations[features.AnnotationLibseccompVersion] = fmt.Sprintf("%d.%d.%d", major, minor, patch)
		}

		enc := json.NewEncoder(context.App.Writer)
		enc.SetIndent("", "    ")
		return enc.Encode(feat)
	},
}
