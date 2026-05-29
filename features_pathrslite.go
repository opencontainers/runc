//go:build !libpathrs

package main

import (
	"github.com/opencontainers/runtime-spec/specs-go/features"
)

func pathrsVersionString() string {
	return ""
}

func featurePathrsVersion(*features.Features) {}
