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
	"bufio"
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/devices"
)

func TestDeviceEmulatorLoad(t *testing.T) {
	tests := []struct {
		name, list string
		expected   *emulator
	}{
		{
			name: "BlacklistMode",
			list: `a *:* rwm`,
			expected: &emulator{
				defaultAllow: true,
			},
		},
		{
			name: "WhitelistBasic",
			list: `c 4:2 rw`,
			expected: &emulator{
				defaultAllow: false,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 4,
						minor: 2,
					}: devices.Permissions("rw"),
				},
			},
		},
		{
			name: "WhitelistWildcard",
			list: `b 0:* m`,
			expected: &emulator{
				defaultAllow: false,
				rules: deviceRules{
					{
						node:  devices.BlockDevice,
						major: 0,
						minor: devices.Wildcard,
					}: devices.Permissions("m"),
				},
			},
		},
		{
			name: "WhitelistDuplicate",
			list: `c *:* rwm
c 1:1 r`,
			expected: &emulator{
				defaultAllow: false,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: devices.Wildcard,
						minor: devices.Wildcard,
					}: devices.Permissions("rwm"),
					// To match the kernel, we allow redundant rules.
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 1,
					}: devices.Permissions("r"),
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
			expected: &emulator{
				defaultAllow: false,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: devices.Wildcard,
						minor: devices.Wildcard,
					}: devices.Permissions("m"),
					{
						node:  devices.BlockDevice,
						major: devices.Wildcard,
						minor: devices.Wildcard,
					}: devices.Permissions("m"),
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 3,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 7,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 8,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 9,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 5,
						minor: 0,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 5,
						minor: 2,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 136,
						minor: devices.Wildcard,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 10,
						minor: 200,
					}: devices.Permissions("rwm"),
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
			emu, err := emulatorFromList(list)
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
		rule           devices.Rule
		base, expected *emulator
	}{
		// Switch between default modes.
		{
			name: "SwitchToOtherMode",
			rule: devices.Rule{
				Type:        devices.WildcardDevice,
				Major:       devices.Wildcard,
				Minor:       devices.Wildcard,
				Permissions: devices.Permissions("rwm"),
				Allow:       !baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: devices.Wildcard,
						minor: devices.Wildcard,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 1,
					}: devices.Permissions("r"),
				},
			},
			expected: &emulator{
				defaultAllow: !baseDefaultAllow,
				rules:        nil,
			},
		},
		{
			name: "SwitchToSameModeNoop",
			rule: devices.Rule{
				Type:        devices.WildcardDevice,
				Major:       devices.Wildcard,
				Minor:       devices.Wildcard,
				Permissions: devices.Permissions("rwm"),
				Allow:       baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules:        nil,
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules:        nil,
			},
		},
		{
			name: "SwitchToSameMode",
			rule: devices.Rule{
				Type:        devices.WildcardDevice,
				Major:       devices.Wildcard,
				Minor:       devices.Wildcard,
				Permissions: devices.Permissions("rwm"),
				Allow:       baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: devices.Wildcard,
						minor: devices.Wildcard,
					}: devices.Permissions("rwm"),
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 1,
					}: devices.Permissions("r"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules:        nil,
			},
		},
		// Rule addition logic.
		{
			name: "RuleAdditionBasic",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: devices.Permissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 2,
						minor: 1,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 2,
						minor: 1,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("rm"),
				},
			},
		},
		{
			name: "RuleAdditionBasicDuplicate",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: devices.Permissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: devices.Wildcard,
					}: devices.Permissions("rwm"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: devices.Wildcard,
					}: devices.Permissions("rwm"),
					// To match the kernel, we allow redundant rules.
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("rm"),
				},
			},
		},
		{
			name: "RuleAdditionBasicDuplicateNoop",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: devices.Permissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("rm"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("rm"),
				},
			},
		},
		{
			name: "RuleAdditionMerge",
			rule: devices.Rule{
				Type:        devices.BlockDevice,
				Major:       5,
				Minor:       12,
				Permissions: devices.Permissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 2,
						minor: 1,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 5,
						minor: 12,
					}: devices.Permissions("rw"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 2,
						minor: 1,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 5,
						minor: 12,
					}: devices.Permissions("rwm"),
				},
			},
		},
		{
			name: "RuleAdditionMergeWildcard",
			rule: devices.Rule{
				Type:        devices.BlockDevice,
				Major:       5,
				Minor:       devices.Wildcard,
				Permissions: devices.Permissions("rm"),
				Allow:       !baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 2,
						minor: 1,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 5,
						minor: devices.Wildcard,
					}: devices.Permissions("rw"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 2,
						minor: 1,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 5,
						minor: devices.Wildcard,
					}: devices.Permissions("rwm"),
				},
			},
		},
		{
			name: "RuleAdditionMergeNoop",
			rule: devices.Rule{
				Type:        devices.BlockDevice,
				Major:       5,
				Minor:       12,
				Permissions: devices.Permissions("r"),
				Allow:       !baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 2,
						minor: 1,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 5,
						minor: 12,
					}: devices.Permissions("rw"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 2,
						minor: 1,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 5,
						minor: 12,
					}: devices.Permissions("rw"),
				},
			},
		},
		// Rule removal logic.
		{
			name: "RuleRemovalBasic",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: devices.Permissions("rm"),
				Allow:       baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("rm"),
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
		},
		{
			name: "RuleRemovalNonexistent",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       4,
				Minor:       1,
				Permissions: devices.Permissions("rw"),
				Allow:       baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
		},
		{
			name: "RuleRemovalFull",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: devices.Permissions("rw"),
				Allow:       baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("w"),
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
		},
		{
			name: "RuleRemovalPartial",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: devices.Permissions("r"),
				Allow:       baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("rm"),
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("m"),
					{
						node:  devices.BlockDevice,
						major: 1,
						minor: 5,
					}: devices.Permissions("r"),
				},
			},
		},
		// Check our non-canonical behaviour when it comes to try to "punch
		// out" holes in a wildcard rule.
		{
			name: "RuleRemovalWildcardPunchoutImpossible",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: devices.Permissions("r"),
				Allow:       baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: devices.Wildcard,
					}: devices.Permissions("rm"),
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("r"),
				},
			},
			expected: nil,
		},
		{
			name: "RuleRemovalWildcardPunchoutPossible",
			rule: devices.Rule{
				Type:        devices.CharDevice,
				Major:       42,
				Minor:       1337,
				Permissions: devices.Permissions("r"),
				Allow:       baseDefaultAllow,
			},
			base: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: devices.Wildcard,
					}: devices.Permissions("wm"),
					{
						node:  devices.CharDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("r"),
				},
			},
			expected: &emulator{
				defaultAllow: baseDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: devices.Wildcard,
					}: devices.Permissions("wm"),
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
		source, target *emulator
		expected       []*devices.Rule
	}{
		// No-op changes.
		{
			name: "Noop",
			source: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: devices.Wildcard,
					}: devices.Permissions("wm"),
				},
			},
			target: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 42,
						minor: devices.Wildcard,
					}: devices.Permissions("wm"),
				},
			},
			// Identical white-lists produce no extra rules.
			expected: nil,
		},
		// Switching modes.
		{
			name: "SwitchToOtherMode",
			source: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rwm"),
				},
			},
			target: &emulator{
				defaultAllow: !sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.BlockDevice,
						major: 42,
						minor: devices.Wildcard,
					}: devices.Permissions("wm"),
				},
			},
			expected: []*devices.Rule{
				// Clear-all rule.
				{
					Type:        devices.WildcardDevice,
					Major:       devices.Wildcard,
					Minor:       devices.Wildcard,
					Permissions: devices.Permissions("rwm"),
					Allow:       !sourceDefaultAllow,
				},
				// The actual rule-set.
				{
					Type:        devices.BlockDevice,
					Major:       42,
					Minor:       devices.Wildcard,
					Permissions: devices.Permissions("wm"),
					Allow:       sourceDefaultAllow,
				},
			},
		},
		// Rule changes.
		{
			name: "RuleAddition",
			source: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rwm"),
				},
			},
			target: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("rwm"),
				},
			},
			expected: []*devices.Rule{
				{
					Type:        devices.BlockDevice,
					Major:       42,
					Minor:       1337,
					Permissions: devices.Permissions("rwm"),
					Allow:       !sourceDefaultAllow,
				},
			},
		},
		{
			name: "RuleRemoval",
			source: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 42,
						minor: 1337,
					}: devices.Permissions("rwm"),
				},
			},
			target: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rwm"),
				},
			},
			expected: []*devices.Rule{
				{
					Type:        devices.BlockDevice,
					Major:       42,
					Minor:       1337,
					Permissions: devices.Permissions("rwm"),
					Allow:       sourceDefaultAllow,
				},
			},
		},
		{
			name: "RuleMultipleAdditionRemoval",
			source: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rwm"),
					{
						node:  devices.BlockDevice,
						major: 3,
						minor: 9,
					}: devices.Permissions("rw"),
				},
			},
			target: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rwm"),
				},
			},
			expected: []*devices.Rule{
				{
					Type:        devices.BlockDevice,
					Major:       3,
					Minor:       9,
					Permissions: devices.Permissions("rw"),
					Allow:       sourceDefaultAllow,
				},
			},
		},
		// Modifying the access permissions.
		{
			name: "RulePartialAddition",
			source: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("r"),
				},
			},
			target: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rwm"),
				},
			},
			expected: []*devices.Rule{
				{
					Type:        devices.CharDevice,
					Major:       1,
					Minor:       2,
					Permissions: devices.Permissions("wm"),
					Allow:       !sourceDefaultAllow,
				},
			},
		},
		{
			name: "RulePartialRemoval",
			source: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rw"),
				},
			},
			target: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("w"),
				},
			},
			expected: []*devices.Rule{
				{
					Type:        devices.CharDevice,
					Major:       1,
					Minor:       2,
					Permissions: devices.Permissions("r"),
					Allow:       sourceDefaultAllow,
				},
			},
		},
		{
			name: "RulePartialBoth",
			source: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rw"),
				},
			},
			target: &emulator{
				defaultAllow: sourceDefaultAllow,
				rules: deviceRules{
					{
						node:  devices.CharDevice,
						major: 1,
						minor: 2,
					}: devices.Permissions("rm"),
				},
			},
			expected: []*devices.Rule{
				{
					Type:        devices.CharDevice,
					Major:       1,
					Minor:       2,
					Permissions: devices.Permissions("w"),
					Allow:       sourceDefaultAllow,
				},
				{
					Type:        devices.CharDevice,
					Major:       1,
					Minor:       2,
					Permissions: devices.Permissions("m"),
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
				test.expected = []*devices.Rule{{
					Type:        devices.WildcardDevice,
					Major:       devices.Wildcard,
					Minor:       devices.Wildcard,
					Permissions: devices.Permissions("rwm"),
					Allow:       test.target.defaultAllow,
				}}
				for _, rule := range test.target.rules.orderedEntries() {
					test.expected = append(test.expected, &devices.Rule{
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

func BenchmarkParseLine(b *testing.B) {
	list := `c *:* m
b *:* m
c 1:3 rwm
c 1:5 rwm
c 1:7 rwm
c 1:8 rwm
c 1:9 rwm
c 5:0 rwm
c 5:2 rwm
c 136:* rwm
c 10:200 rwm`

	var r *deviceRule
	var err error
	for i := 0; i < b.N; i++ {
		s := bufio.NewScanner(strings.NewReader(list))
		for s.Scan() {
			line := s.Text()
			r, err = parseLine(line)
		}
		if err := s.Err(); err != nil {
			b.Fatal(err)
		}
	}
	b.Logf("rule: %v, err: %v", r, err)
}
