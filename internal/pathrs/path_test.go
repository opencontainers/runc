// SPDX-License-Identifier: Apache-2.0
/*
 * Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2024-2025 SUSE LLC
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

package pathrs

import "testing"

func TestIsLexicallyInRoot(t *testing.T) {
	for _, test := range []struct {
		name       string
		root, path string
		expected   bool
	}{
		{"Equal1", "/foo", "/foo", true},
		{"Equal2", "/bar/baz", "/bar/baz", true},
		{"Equal3", "/bar/baz/", "/bar/baz/", true},
		{"Root", "/", "/foo/bar", true},
		{"Root-Equal", "/", "/", true},
		{"InRoot-Basic1", "/foo/bar", "/foo/bar/baz/abcd", true},
		{"InRoot-Basic2", "/a/b/c/d", "/a/b/c/d/e/f/g/h", true},
		{"InRoot-Long", "/var/lib/docker/container/1234abcde/rootfs", "/var/lib/docker/container/1234abcde/rootfs/a/b/c", true},
		{"InRoot-TrailingSlash1", "/foo/bar/", "/foo/bar", true},
		{"InRoot-TrailingSlash2", "/foo/", "/foo/bar/baz/boop", true},
		{"NotInRoot-Basic1", "/foo", "/bar", false},
		{"NotInRoot-Basic2", "/foo", "/bar", false},
		{"NotInRoot-Basic3", "/foo/bar/baz", "/foo/boo/baz/abc", false},
		{"NotInRoot-Long", "/var/lib/docker/container/1234abcde/rootfs", "/a/b/c", false},
		{"NotInRoot-Tricky1", "/foo/bar", "/foo/bara", false},
		{"NotInRoot-Tricky2", "/foo/bar", "/foo/ba/r", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := IsLexicallyInRoot(test.root, test.path)
			if test.expected != got {
				t.Errorf("IsLexicallyInRoot(%q, %q) = %v (expected %v)", test.root, test.path, got, test.expected)
			}
		})
	}
}
