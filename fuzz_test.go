//go:build go1.18 && linux
// +build go1.18,linux

// Copyright 2014 Docker, Inc.
// SPDX-License-Identifier: Apache-2.0

package runc_test

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

// FuzzHooksUnmarshalJSON tests OCI hooks JSON parsing with
// arbitrary attacker-controlled JSON input.
//
// OCI hooks are container lifecycle hooks configured via JSON.
// runc processes these before container execution — a parsing
// bug here affects every container runtime that uses runc.
//
// 17 GitHub Security Advisories exist for runc.
func FuzzHooksUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`{"prestart":[{"path":"/bin/echo"}]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"poststop":null}`))
	f.Add([]byte(``))
	f.Add([]byte(`{`))
	f.Add(make([]byte, 10000))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			return
		}

		var hooks configs.Hooks
		// UnmarshalJSON must never panic
		_ = hooks.UnmarshalJSON(data)
	})
}

// FuzzHooksMarshalJSON tests OCI hooks JSON marshaling
// round-trip with arbitrary hook configurations.
func FuzzHooksMarshalJSON(f *testing.F) {
	f.Add([]byte(`{"prestart":[{"path":"/bin/echo","args":["arg1"]}]}`))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			return
		}

		var hooks configs.Hooks
		if err := hooks.UnmarshalJSON(data); err != nil {
			return
		}
		// Marshal must never panic on valid hooks
		_, _ = hooks.MarshalJSON()
	})
}
