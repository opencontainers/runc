package fs2

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func statPSI(dirPath string, file string, stats *cgroups.PSIStats) error {
	f, err := cgroups.OpenFile(dirPath, file, os.O_RDONLY)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTSUP) {
			// open *.pressure file returns
			// - ErrNotExist when kernel < 4.20 or CONFIG_PSI is disabled
			// - ENOTSUP when we requires psi=1 in kernel command line to enable PSI support
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		switch parts[0] {
		case "some":
			stats.Some, err = parsePSIData(parts[1:])
			if err != nil {
				return err
			}
		case "full":
			stats.Full, err = parsePSIData(parts[1:])
			if err != nil {
				return err
			}
		}
	}
	err = sc.Err()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTSUP) {
			return nil
		}
		return &parseError{Path: dirPath, File: file, Err: err}
	}
	return nil
}

func parsePSIData(psi []string) (*cgroups.PSIData, error) {
	var (
		data = &cgroups.PSIData{}
		err  error
	)
	for _, f := range psi {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid PSI data: %q", f)
		}
		switch kv[0] {
		case "avg10", "avg60", "avg300":
			v, err := strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid PSI value: %q", f)
			}
			switch kv[0] {
			case "avg10":
				data.Avg10 = v
			case "avg60":
				data.Avg60 = v
			case "avg300":
				data.Avg300 = v
			}
		case "total":
			data.Total, err = strconv.ParseUint(kv[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid PSI value: %q", f)
			}
		}
	}
	return data, nil
}
