// +build gofuzz

package fscommon

import (
	gofuzzheaders "github.com/AdaLogics/go-fuzz-headers"
	securejoin "github.com/cyphar/filepath-securejoin"
)

func FuzzSecurejoin(data []byte) int {
	c := gofuzzheaders.NewConsumer(data)
	dir, err := c.GetString()
	if err != nil {
		return 0
	}
	file, err := c.GetString()
	if err != nil {
		return 0
	}
	_, err = securejoin.SecureJoin(dir, file)
	if err != nil {
		return 0
	}
	return 1
}
