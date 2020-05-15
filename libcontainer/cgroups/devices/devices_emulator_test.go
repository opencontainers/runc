// SPDX-License-Identifier: Apache-2.0
/*
 * Copyright (C) 2020 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2020 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package devices

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestDeviceEmulatorLoad(t *testing.T) {
	tests := []struct {
		name, list string
		expected   *Emulator
	}{
		{
			name: "BlacklistMode",
			list: `a *:* rwm`,
			expected: &Emulator{
				defaultAllow: true,
			},
		},
		{
			name: "WhitelistBasic",
			list: `c 4:2 rw`,
			expected: &Emulator{
				defaultAllow: false,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 4,
						minor: 2,
					}: configs.DevicePermissions("rw"),
				},
			},
		},
		{
			name: "WhitelistWildcard",
			list: `b 0:* m`,
			expected: &Emulator{
				defaultAllow: false,
				rules: deviceRules{
					{
						node:  configs.BlockDevice,
						major: 0,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("m"),
				},
			},
		},
		{
			name: "WhitelistDuplicate",
			list: `c *:* rwm
c 1:1 r`,
			expected: &Emulator{
				defaultAllow: false,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: configs.Wildcard,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rwm"),
					// To match the kernel, we allow redundant rules.
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 1,
					}: configs.DevicePermissions("r"),
				},
			},
		},
		{
			name: "WhitelistComplicated",
			list: `c *:* m
b *:* m
c 1:3 rwm
c 1:5 rwm
c 1:7 rwm
c 1:8 rwm
c 1:9 rwm
c 5:0 rwm
c 5:2 rwm
c 136:* rwm
c 10:200 rwm`,
			expected: &Emulator{
				defaultAllow: false,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: configs.Wildcard,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("m"),
					{
						node:  configs.BlockDevice,
						major: configs.Wildcard,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("m"),
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 3,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 7,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 8,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 9,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 5,
						minor: 0,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 5,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 136,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 10,
						minor: 200,
					}: configs.DevicePermissions("rwm"),
				},
			},
		},
		// Some invalid lists.
		{
			name:     "InvalidFieldNumber",
			list:     `b 1:0`,
			expected: nil,
		},
		{
			name:     "InvalidDeviceType",
			list:     `p *:* rwm`,
			expected: nil,
		},
		{
			name:     "InvalidMajorNumber1",
			list:     `p -1:3 rwm`,
			expected: nil,
		},
		{
			name:     "InvalidMajorNumber2",
			list:     `c foo:27 rwm`,
			expected: nil,
		},
		{
			name:     "InvalidMinorNumber1",
			list:     `b 1:-4 rwm`,
			expected: nil,
		},
		{
			name:     "InvalidMinorNumber2",
			list:     `b 1:foo rwm`,
			expected: nil,
		},
		{
			name:     "InvalidPermissions",
			list:     `b 1:7 rwk`,
			expected: nil,
		},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(test.name, func(t *testing.T) {
			list := bytes.NewBufferString(test.list)
			emu, err := EmulatorFromList(list)
			if err != nil && test.expected != nil {
				t.Fatalf("unexpected failure when creating emulator: %v", err)
			} else if err == nil && test.expected == nil {
				t.Fatalf("unexpected success when creating emulator: %#v", emu)
			}

			if !reflect.DeepEqual(emu, test.expected) {
				t.Errorf("final emulator state mismatch: %#v != %#v", emu, test.expected)
			}
		})
	}
}

func testDeviceEmulatorApply(t *testing.T, baseDefaultAllow bool) {
	tests := []struct {
		name           string
		rule           configs.DeviceRule
		base, expected *Emulator
	}{
		// Switch between default modes.
		{
			name: "SwitchToOtherMode",
			rule: configs.DeviceRule{
				Type:        configs.WildcardDevice,
				Major:       configs.Wildcard,
				Minor:       configs.Wildcard,
				Permissions: configs.DevicePermissions("rwm"),
				Allow:       !baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: configs.Wildcard,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 1,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: &Emulator{
				defaultAllow: !baseDefaultAllow,
				rules:        nil,
			},
		},
		{
			name: "SwitchToSameModeNoop",
			rule: configs.DeviceRule{
				Type:        configs.WildcardDevice,
				Major:       configs.Wildcard,
				Minor:       configs.Wildcard,
				Permissions: configs.DevicePermissions("rwm"),
				Allow:       baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules:        nil,
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules:        nil,
			},
		},
		{
			name: "SwitchToSameMode",
			rule: configs.DeviceRule{
				Type:        configs.WildcardDevice,
				Major:       configs.Wildcard,
				Minor:       configs.Wildcard,
				Permissions: configs.DevicePermissions("rwm"),
				Allow:       baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: configs.Wildcard,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 1,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules:        nil,
			},
		},
		// Rule addition logic.
		{
			name: "RuleAdditionBasic",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: configs.DevicePermissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 2,
						minor: 1,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 2,
						minor: 1,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("rm"),
				},
			},
		},
		{
			name: "RuleAdditionBasicDuplicate",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: configs.DevicePermissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rwm"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rwm"),
					// To match the kernel, we allow redundant rules.
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("rm"),
				},
			},
		},
		{
			name: "RuleAdditionBasicDuplicateNoop",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: configs.DevicePermissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("rm"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("rm"),
				},
			},
		},
		{
			name: "RuleAdditionMerge",
			rule: configs.DeviceRule{
				Type:        configs.BlockDevice,
				Major:       5,
				Minor:       12,
				Permissions: configs.DevicePermissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 2,
						minor: 1,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 5,
						minor: 12,
					}: configs.DevicePermissions("rw"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 2,
						minor: 1,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 5,
						minor: 12,
					}: configs.DevicePermissions("rwm"),
				},
			},
		},
		{
			name: "RuleAdditionMergeWildcard",
			rule: configs.DeviceRule{
				Type:        configs.BlockDevice,
				Major:       5,
				Minor:       configs.Wildcard,
				Permissions: configs.DevicePermissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 2,
						minor: 1,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 5,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rw"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 2,
						minor: 1,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 5,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rwm"),
				},
			},
		},
		{
			name: "RuleAdditionMergeNoop",
			rule: configs.DeviceRule{
				Type:        configs.BlockDevice,
				Major:       5,
				Minor:       12,
				Permissions: configs.DevicePermissions("r"),
				Allow:       !baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 2,
						minor: 1,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 5,
						minor: 12,
					}: configs.DevicePermissions("rw"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 2,
						minor: 1,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 5,
						minor: 12,
					}: configs.DevicePermissions("rw"),
				},
			},
		},
		// Rule removal logic.
		{
			name: "RuleRemovalBasic",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: configs.DevicePermissions("rm"),
				Allow:       baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("rm"),
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
		},
		{
			name: "RuleRemovalNonexistent",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       4,
				Minor:       1,
				Permissions: configs.DevicePermissions("rw"),
				Allow:       baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
		},
		{
			name: "RuleRemovalFull",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: configs.DevicePermissions("rw"),
				Allow:       baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("w"),
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
		},
		{
			name: "RuleRemovalPartial",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: configs.DevicePermissions("r"),
				Allow:       baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("rm"),
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("m"),
					{
						node:  configs.BlockDevice,
						major: 1,
						minor: 5,
					}: configs.DevicePermissions("r"),
				},
			},
		},
		// Check our non-canonical behaviour when it comes to try to "punch
		// out" holes in a wildcard rule.
		{
			name: "RuleRemovalWildcardPunchoutImpossible",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: configs.DevicePermissions("r"),
				Allow:       baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("rm"),
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: nil,
		},
		{
			name: "RuleRemovalWildcardPunchoutPossible",
			rule: configs.DeviceRule{
				Type:        configs.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: configs.DevicePermissions("r"),
				Allow:       baseDefaultAllow,
			},
			base: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("wm"),
					{
						node:  configs.CharDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("r"),
				},
			},
			expected: &Emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("wm"),
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			err := test.base.Apply(test.rule)
			if err != nil && test.expected != nil {
				t.Fatalf("unexpected failure when applying apply rule: %v", err)
			} else if err == nil && test.expected == nil {
				t.Fatalf("unexpected success when applying apply rule: %#v", test.base)
			}

			if test.expected != nil && !reflect.DeepEqual(test.base, test.expected) {
				t.Errorf("final emulator state mismatch: %#v != %#v", test.base, test.expected)
			}
		})
	}
}

func TestDeviceEmulatorWhitelistApply(t *testing.T) {
	testDeviceEmulatorApply(t, false)
}

func TestDeviceEmulatorBlacklistApply(t *testing.T) {
	testDeviceEmulatorApply(t, true)
}

func testDeviceEmulatorTransition(t *testing.T, sourceDefaultAllow bool) {
	tests := []struct {
		name           string
		source, target *Emulator
		expected       []*configs.DeviceRule
	}{
		// No-op changes.
		{
			name: "Noop",
			source: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("wm"),
				},
			},
			target: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 42,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("wm"),
				},
			},
			// Identical white-lists produce no extra rules.
			expected: nil,
		},
		// Switching modes.
		{
			name: "SwitchToOtherMode",
			source: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
				},
			},
			target: &Emulator{
				defaultAllow: !sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.BlockDevice,
						major: 42,
						minor: configs.Wildcard,
					}: configs.DevicePermissions("wm"),
				},
			},
			expected: []*configs.DeviceRule{
				// Clear-all rule.
				{
					Type:        configs.WildcardDevice,
					Major:       configs.Wildcard,
					Minor:       configs.Wildcard,
					Permissions: configs.DevicePermissions("rwm"),
					Allow:       !sourceDefaultAllow,
				},
				// The actual rule-set.
				{
					Type:        configs.BlockDevice,
					Major:       42,
					Minor:       configs.Wildcard,
					Permissions: configs.DevicePermissions("wm"),
					Allow:       sourceDefaultAllow,
				},
			},
		},
		// Rule changes.
		{
			name: "RuleAddition",
			source: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
				},
			},
			target: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("rwm"),
				},
			},
			expected: []*configs.DeviceRule{
				{
					Type:        configs.BlockDevice,
					Major:       42,
					Minor:       1337,
					Permissions: configs.DevicePermissions("rwm"),
					Allow:       !sourceDefaultAllow,
				},
			},
		},
		{
			name: "RuleRemoval",
			source: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 42,
						minor: 1337,
					}: configs.DevicePermissions("rwm"),
				},
			},
			target: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
				},
			},
			expected: []*configs.DeviceRule{
				{
					Type:        configs.BlockDevice,
					Major:       42,
					Minor:       1337,
					Permissions: configs.DevicePermissions("rwm"),
					Allow:       sourceDefaultAllow,
				},
			},
		},
		{
			name: "RuleMultipleAdditionRemoval",
			source: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
					{
						node:  configs.BlockDevice,
						major: 3,
						minor: 9,
					}: configs.DevicePermissions("rw"),
				},
			},
			target: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
				},
			},
			expected: []*configs.DeviceRule{
				{
					Type:        configs.BlockDevice,
					Major:       3,
					Minor:       9,
					Permissions: configs.DevicePermissions("rw"),
					Allow:       sourceDefaultAllow,
				},
			},
		},
		// Modifying the access permissions.
		{
			name: "RulePartialAddition",
			source: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("r"),
				},
			},
			target: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rwm"),
				},
			},
			expected: []*configs.DeviceRule{
				{
					Type:        configs.CharDevice,
					Major:       1,
					Minor:       2,
					Permissions: configs.DevicePermissions("wm"),
					Allow:       !sourceDefaultAllow,
				},
			},
		},
		{
			name: "RulePartialRemoval",
			source: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rw"),
				},
			},
			target: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("w"),
				},
			},
			expected: []*configs.DeviceRule{
				{
					Type:        configs.CharDevice,
					Major:       1,
					Minor:       2,
					Permissions: configs.DevicePermissions("r"),
					Allow:       sourceDefaultAllow,
				},
			},
		},
		{
			name: "RulePartialBoth",
			source: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rw"),
				},
			},
			target: &Emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  configs.CharDevice,
						major: 1,
						minor: 2,
					}: configs.DevicePermissions("rm"),
				},
			},
			expected: []*configs.DeviceRule{
				{
					Type:        configs.CharDevice,
					Major:       1,
					Minor:       2,
					Permissions: configs.DevicePermissions("w"),
					Allow:       sourceDefaultAllow,
				},
				{
					Type:        configs.CharDevice,
					Major:       1,
					Minor:       2,
					Permissions: configs.DevicePermissions("m"),
					Allow:       !sourceDefaultAllow,
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// If we are in black-list mode, we need to prepend the relevant
			// clear-all rule (the expected rule lists are written with
			// white-list mode in mind), and then make a full copy of the
			// target rules.
			if sourceDefaultAllow && test.source.defaultAllow == test.target.defaultAllow {
				test.expected = []*configs.DeviceRule{{
					Type:        configs.WildcardDevice,
					Major:       configs.Wildcard,
					Minor:       configs.Wildcard,
					Permissions: configs.DevicePermissions("rwm"),
					Allow:       test.target.defaultAllow,
				}}
				for _, rule := range test.target.rules.orderedEntries() {
					test.expected = append(test.expected, &configs.DeviceRule{
						Type:        rule.meta.node,
						Major:       rule.meta.major,
						Minor:       rule.meta.minor,
						Permissions: rule.perms,
						Allow:       !test.target.defaultAllow,
					})
				}
			}

			rules, err := test.source.Transition(test.target)
			if err != nil {
				t.Fatalf("unexpected error while calculating transition rules: %#v", err)
			}

			if !reflect.DeepEqual(rules, test.expected) {
				t.Errorf("rules don't match expected set: %#v != %#v", rules, test.expected)
			}

			// Apply the rules to the source to see if it actually transitions
			// correctly. This is all emulated but it's a good thing to
			// double-check.
			for _, rule := range rules {
				if err := test.source.Apply(*rule); err != nil {
					t.Fatalf("error while applying transition rule [%#v]: %v", rule, err)
				}
			}
			if !reflect.DeepEqual(test.source, test.target) {
				t.Errorf("transition incomplete after applying all rules: %#v != %#v", test.source, test.target)
			}
		})
	}
}

func TestDeviceEmulatorTransitionFromBlacklist(t *testing.T) {
	testDeviceEmulatorTransition(t, true)
}

func TestDeviceEmulatorTransitionFromWhitelist(t *testing.T) {
	testDeviceEmulatorTransition(t, false)
}
