//go:build go1.27 && runc_strictspec

package main

import (
	"encoding/json/v2"
	"io"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// decodeSpec is a strict version of the spec decoder, meant for testing only.
// Unlike encoding/json (v1), it uses case-sensitive key matching, and rejects
// unknown keys. This helps to find mistakes in config.json which runc would
// silently ignore, but other runtimes may not.
func decodeSpec(r io.Reader, spec **specs.Spec) error {
	return json.UnmarshalRead(r, spec, json.RejectUnknownMembers(true))
}
