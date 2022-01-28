package fs2

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func statPSI(dirPath string, file string, stats *cgroups.PSIStats) error {
	f, err := cgroups.OpenFile(dirPath, file, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		switch parts[0] {
		case "some":
			data, err := parsePSIData(parts[1:])
			if err != nil {
				return err
			}
			stats.Some = data
		case "full":
			data, err := parsePSIData(parts[1:])
			if err != nil {
				return err
			}
			stats.Full = data
		}
	}
	if err := sc.Err(); err != nil {
		return &parseError{Path: dirPath, File: file, Err: err}
	}
	return nil
}

func parsePSIData(psi []string) (data cgroups.PSIData, err error) {
	for _, f := range psi {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			return data, fmt.Errorf("invalid psi data: %q", f)
		}
		switch kv[0] {
		case "avg10":
			v, err := strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return data, fmt.Errorf("invalid psi value: %q", f)
			}
			data.Avg10 = v
		case "avg60":
			v, err := strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return data, fmt.Errorf("invalid psi value: %q", f)
			}
			data.Avg60 = v
		case "avg300":
			v, err := strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return data, fmt.Errorf("invalid psi value: %q", f)
			}
			data.Avg300 = v
		case "total":
			v, err := strconv.ParseUint(kv[1], 10, 64)
			if err != nil {
				return data, fmt.Errorf("invalid psi value: %q", f)
			}
			data.Total = v
		}
	}
	return data, nil
}
