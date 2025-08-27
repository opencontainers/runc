package validate

import (
	"sync"

	"github.com/opencontainers/runc/libcontainer/intelrdt"
)

// Cache the result of intelrdt IsEnabled functions to avoid repeated sysfs
// access and enable mocking for unit tests.
type intelRdtStatus struct {
	sync.Once
	rdtEnabled bool
	catEnabled bool
	mbaEnabled bool
}

var intelRdt = &intelRdtStatus{}

func (i *intelRdtStatus) init() {
	i.Do(func() {
		i.rdtEnabled = intelrdt.IsEnabled()
		i.catEnabled = intelrdt.IsCATEnabled()
		i.mbaEnabled = intelrdt.IsMBAEnabled()
	})
}

func (i *intelRdtStatus) isEnabled() bool {
	i.init()
	return i.rdtEnabled
}

func (i *intelRdtStatus) isCATEnabled() bool {
	i.init()
	return i.catEnabled
}

func (i *intelRdtStatus) isMBAEnabled() bool {
	i.init()
	return i.mbaEnabled
}
