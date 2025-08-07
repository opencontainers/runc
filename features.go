package main

import (
	"encoding/json"
	"fmt"

	"github.com/opencontainers/runc/libcontainer/capabilities"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/seccomp"
	"github.com/opencontainers/runc/libcontainer/specconv"
	runcfeatures "github.com/opencontainers/runc/types/features"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-spec/specs-go/features"
	"github.com/urfave/cli"
)

var featuresCommand = cli.Command{
	Name:      "features",
	Usage:     "show the enabled features",
	ArgsUsage: "",
	Description: `Show the enabled features.
   The result is parsable as a JSON.
   See https://github.com/opencontainers/runtime-spec/blob/main/features.md for the type definition.
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
				runcfeatures.AnnotationRuncVersion:           version,
				runcfeatures.AnnotationRuncCommit:            gitCommit,
				runcfeatures.AnnotationRuncCheckpointEnabled: "true",
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
					Rdma:        &t,
				},
				Apparmor: &features.Apparmor{
					Enabled: &t,
				},
				Selinux: &features.Selinux{
					Enabled: &t,
				},
				IntelRdt: &features.IntelRdt{
					Enabled: &t,
				},
				MountExtensions: &features.MountExtensions{
					IDMap: &features.IDMap{
						Enabled: &t,
					},
				},
				NetDevices: &features.NetDevices{
					Enabled: &t,
				},
			},
			PotentiallyUnsafeConfigAnnotations: []string{
				"bundle",
				"org.systemd.property.", // prefix form
				"org.criu.config",
			},
		}

		if seccomp.Enabled {
			feat.Linux.Seccomp = &features.Seccomp{
				Enabled:        &t,
				Actions:        seccomp.KnownActions(),
				Operators:      seccomp.KnownOperators(),
				Archs:          seccomp.KnownArchs(),
				KnownFlags:     seccomp.KnownFlags(),
				SupportedFlags: seccomp.SupportedFlags(),
			}
			major, minor, patch := seccomp.Version()
			feat.Annotations[runcfeatures.AnnotationLibseccompVersion] = fmt.Sprintf("%d.%d.%d", major, minor, patch)
		}

		enc := json.NewEncoder(context.App.Writer)
		enc.SetIndent("", "    ")
		return enc.Encode(feat)
	},
}
