package landlock

import (
	"fmt"

	ll "github.com/landlock-lsm/go-landlock/landlock"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func Apply(cfg *configs.LandlockConfig) error {
	if cfg == nil {
		return nil
	}

	// Choose ABI + fallback policy
	var c ll.Config
	switch cfg.Mode {
	case "best-effort":
		c = ll.V5.BestEffort() // V5 covers FS+NET+ioctl-dev; will step down automatically
	case "enforce":
		c = ll.V5 // or ll.V6 once you use scopes
	default:
		c = ll.V5.BestEffort()
	}

	var rules []ll.Rule

	if len(cfg.RoDirs) > 0 {
		rules = append(rules, ll.RODirs(cfg.RoDirs...))
	}
	if len(cfg.RwDirs) > 0 {
		rules = append(rules, ll.RWDirs(cfg.RwDirs...))
	}
	for _, d := range cfg.WithRefer {
		rules = append(rules, ll.RWDirs(d).WithRefer())
	}
	for _, d := range cfg.IoctlDev {
		rules = append(rules, ll.RODirs(d).WithIoctlDev())
	}

	for _, p := range cfg.BindTCP {
		rules = append(rules, ll.BindTCP(p))
	}
	for _, p := range cfg.ConnectTCP {
		rules = append(rules, ll.ConnectTCP(p))
	}

	// This sets PR_SET_NO_NEW_PRIVS as needed and then restricts self.
	// The library internally queries ABI and degrades if BestEffort().
	if err := c.Restrict(rules...); err != nil {
		if cfg.Mode == "enforce" {
			return fmt.Errorf("landlock enforce failed: %w", err)
		}
	}
	return nil
}
