//go:build !linux

package landlock

import "fmt"

func restrict(c Config, rules ...Rule) error {
	if c.bestEffort {
		return nil // Fallback to "nothing"
	}
	return fmt.Errorf("missing kernel Landlock support. Landlock is only supported on Linux")
}
