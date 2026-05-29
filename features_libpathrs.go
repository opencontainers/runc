//go:build libpathrs

package main

import (
	"cyphar.com/go-pathrs"
	"github.com/opencontainers/runtime-spec/specs-go/features"

	runcfeatures "github.com/opencontainers/runc/types/features"
)

func pathrsVersionString() string {
	info, err := pathrs.LibraryVersion()
	if err != nil {
		panic(err) // should never happen
	}
	return info.VersionString
}

func featurePathrsVersion(feat *features.Features) {
	feat.Annotations[runcfeatures.AnnotationLibpathrsVersion] = pathrsVersionString()
}
