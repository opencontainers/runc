package main

import (
	"encoding/json"
	"fmt"

	"github.com/opencontainers/runc/libcontainer/capabilities"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/seccomp"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/types/features"
	"github.com/opencontainers/runtime-spec/specs-go"
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

		t := true

		feat := features.Features{
			OCIVersionMin: "1.0.0",
			OCIVersionMax: specs.Version,
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
					V1:          &t,
					V2:          &t,
					Systemd:     &t,
					SystemdUser: &t,
				},
				Apparmor: &features.Apparmor{
					Enabled: &t,
				},
				Selinux: &features.Selinux{
					Enabled: &t,
				},
			},
		}

		if seccomp.Enabled {
			feat.Linux.Seccomp = &features.Seccomp{
				Enabled:   &t,
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
