/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package capabilities

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/syndtr/gocapability/capability"
)

// fromBitmap parses an uint64 bitmap into a capability map. Unknown cap numbers
// are ignored.
func fromBitmap(v uint64) map[string]capability.Cap {
	res := make(map[string]capability.Cap, 63)
	for i := 0; i <= 63; i++ {
		if b := (v >> i) & 0x1; b == 0x1 {
			c := capability.Cap(i)
			if s := c.String(); s != "unknown" {
				res["CAP_"+strings.ToUpper(s)] = c
			}
		}
	}
	return res
}

// parseProcPIDStatus returns uint64 bitmap value from /proc/<PID>/status file
func parseProcPIDStatus(r io.Reader) (map[capability.CapType]uint64, error) {
	res := make(map[capability.CapType]uint64)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		pair := strings.SplitN(line, ":", 2)
		if len(pair) != 2 {
			continue
		}
		k := strings.TrimSpace(pair[0])
		v := strings.TrimSpace(pair[1])
		switch k {
		case "CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb":
			ui64, err := strconv.ParseUint(v, 16, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse line %q", line)
			}
			switch k {
			case "CapInh":
				res[capability.INHERITABLE] = ui64
			case "CapPrm":
				res[capability.PERMITTED] = ui64
			case "CapEff":
				res[capability.EFFECTIVE] = ui64
			case "CapBnd":
				res[capability.BOUNDING] = ui64
			case "CapAmb":
				res[capability.AMBIENT] = ui64
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

var (
	curCaps     map[string]capability.Cap
	curCapsErr  error
	curCapsOnce sync.Once
)

// current returns a map of the effective known caps of the current process.
func current() (map[string]capability.Cap, error) {
	curCapsOnce.Do(func() {
		f, curCapsErr := os.Open("/proc/self/status")
		if curCapsErr != nil {
			return
		}
		defer f.Close()
		caps, curCapsErr := parseProcPIDStatus(f)
		if curCapsErr != nil {
			return
		}
		curCaps = fromBitmap(caps[capability.EFFECTIVE])
	})
	return curCaps, curCapsErr
}
