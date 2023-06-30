package kernelparam

import (
	"io/fs"
	"strings"
	"unicode"
)

type KernelBootParam string

const (
	IsolatedCPUAffinityTransition KernelBootParam = "runc.exec.isolated-cpu-affinity-transition"
	NohzFull                      KernelBootParam = "nohz_full"

	TemporaryIsolatedCPUAffinityTransition  = "temporary"
	DefinitiveIsolatedCPUAffinityTransition = "definitive"
)

// LookupKernelBootParameters returns the selected kernel parameters specified
// in the kernel command line. The parameters are returned as a map of key-value pairs.
func LookupKernelBootParameters(rootFS fs.FS, lookupParameters ...KernelBootParam) (map[KernelBootParam]string, error) {
	cmdline, err := fs.ReadFile(rootFS, "proc/cmdline")
	if err != nil {
		return nil, err
	}

	kernelParameters := make(map[KernelBootParam]string)
	remaining := len(lookupParameters)

	runeKeeper := func(c rune) bool {
		return !unicode.IsPrint(c) || unicode.IsSpace(c)
	}

	for _, parameter := range strings.FieldsFunc(string(cmdline), runeKeeper) {
		if remaining == 0 {
			break
		}
		idx := strings.IndexByte(parameter, '=')
		if idx == -1 {
			continue
		}
		for _, lookupParam := range lookupParameters {
			if lookupParam == KernelBootParam(parameter[:idx]) {
				kernelParameters[lookupParam] = parameter[idx+1:]
				remaining--
				break
			}
		}
	}

	return kernelParameters, nil
}
