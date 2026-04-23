package validate

import (
	"github.com/opencontainers/runc/libcontainer/intelrdt"
)

// Allow mocking for TestValidateIntelRdt.
var (
	intelRdtIsEnabled    = intelrdt.IsEnabled
	intelRdtIsCATEnabled = intelrdt.IsCATEnabled
	intelRdtIsMBAEnabled = intelrdt.IsMBAEnabled
)
