// +build gofuzz

package fs2

import (
	"bytes"
)

func FuzzCgroupReader(data []byte) int {
	r := bytes.NewReader(data)
	_, _ = parseCgroupFromReader(r)
	return 1
}
