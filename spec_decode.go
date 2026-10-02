//go:build !runc_strictspec

package main

import (
	"encoding/json"
	"io"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func decodeSpec(r io.Reader, spec **specs.Spec) error {
	return json.NewDecoder(r).Decode(spec)
}
