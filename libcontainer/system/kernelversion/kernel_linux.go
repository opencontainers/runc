// Package kernelversion provides a method to check whether the running kernel
// version is at least a minimum kernel version.
//
// Deprecated: This package is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
package kernelversion

import (
	"github.com/opencontainers/runc/internal/kernelversion"
)

// KernelVersion holds information about the kernel.
//
// Deprecated: This type is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
type KernelVersion = kernelversion.KernelVersion

// GreaterEqualThan checks if the host's kernel version is greater than, or
// equal to the given kernel version v. Only "kernel version" and "major revision"
// can be specified (e.g., "3.12") and will be taken into account, which means
// that 3.12.25-gentoo and 3.12-1-amd64 are considered equal (kernel: 3, major: 12).
//
// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func GreaterEqualThan(minVersion KernelVersion) (bool, error) {
	return kernelversion.GreaterEqualThan(minVersion)
}
