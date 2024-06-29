//go:build !linux

package userns

// runningInUserNS is a stub for non-Linux systems
// Always returns false
func runningInUserNS() bool {
	return false
}
