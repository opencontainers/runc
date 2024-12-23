package landlock

import ll "github.com/landlock-lsm/go-landlock/landlock/syscall"

type abiInfo struct {
	version           int
	supportedAccessFS AccessFSSet
}

var abiInfos = []abiInfo{
	{
		version:           0,
		supportedAccessFS: 0,
	},
	{
		version:           1,
		supportedAccessFS: (1 << 13) - 1,
	},
}

// getSupportedABIVersion returns the kernel-supported ABI version.
//
// If the ABI version supported by the kernel is higher than the
// newest one known to go-landlock, the highest ABI version known to
// go-landlock is returned.
func getSupportedABIVersion() abiInfo {
	v, err := ll.LandlockGetABIVersion()
	if err != nil {
		v = 0 // ABI version 0 is "no Landlock support".
	}
	if v >= len(abiInfos) {
		v = len(abiInfos) - 1
	}
	return abiInfos[v]
}
