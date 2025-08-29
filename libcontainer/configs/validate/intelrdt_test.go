package validate

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestValidateIntelRdt(t *testing.T) {
	// Call init to trigger the sync.Once and enable overriding the rdt status
	intelRdt.init()

	testCases := []struct {
		name       string
		rdtEnabled bool
		catEnabled bool
		mbaEnabled bool
		config     *configs.IntelRdt
		isErr      bool
	}{
		{
			name:       "rdt not supported, no config",
			rdtEnabled: false,
			config:     nil,
			isErr:      false,
		},
		{
			name:       "rdt not supported, with config",
			rdtEnabled: false,
			config:     &configs.IntelRdt{},
			isErr:      true,
		},
		{
			name:       "empty config",
			rdtEnabled: true,
			config:     &configs.IntelRdt{},
			isErr:      false,
		},
		{
			name:       "root clos",
			rdtEnabled: true,
			config: &configs.IntelRdt{
				ClosID: "/",
			},
			isErr: false,
		},
		{
			name:       "invalid ClosID (.)",
			rdtEnabled: true,
			config: &configs.IntelRdt{
				ClosID: ".",
			},
			isErr: true,
		},
		{
			name:       "invalid ClosID (..)",
			rdtEnabled: true,
			config: &configs.IntelRdt{
				ClosID: "..",
			},
			isErr: true,
		},
		{
			name:       "invalid ClosID (contains /)",
			rdtEnabled: true,
			config: &configs.IntelRdt{
				ClosID: "foo/bar",
			},
			isErr: true,
		},
		{
			name:       "cat not supported",
			rdtEnabled: true,
			catEnabled: false,
			config: &configs.IntelRdt{
				L3CacheSchema: "0=ff",
			},
			isErr: true,
		},
		{
			name:       "mba not supported",
			rdtEnabled: true,
			mbaEnabled: false,
			config: &configs.IntelRdt{
				MemBwSchema: "0=100",
			},
			isErr: true,
		},
		{
			name:       "valid config",
			rdtEnabled: true,
			catEnabled: true,
			mbaEnabled: true,
			config: &configs.IntelRdt{
				ClosID:        "clos-1",
				L3CacheSchema: "0=ff",
				MemBwSchema:   "0=100",
			},
			isErr: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			intelRdt.rdtEnabled = tc.rdtEnabled
			intelRdt.catEnabled = tc.catEnabled
			intelRdt.mbaEnabled = tc.mbaEnabled

			config := &configs.Config{
				Rootfs:   "/var",
				IntelRdt: tc.config,
			}

			err := Validate(config)
			if tc.isErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.isErr && err != nil {
				t.Error(err)
			}
		})
	}
}
