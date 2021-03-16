// +build gofuzz

package libcontainer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func FuzzInit(data []byte) int {
	if len(data) < 5 {
		return -1
	}

	pipe, err := os.OpenFile("pipe.txt", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return -1
	}
	defer pipe.Close()
	defer os.RemoveAll("pipe.txt")

	consoleSocket, err = os.OpenFile("consoleSocket.txt", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return -1
	}
	defer consoleSocket.Close()
	defer os.RemoveAll("consoleSocket.txt")

	var config *initConfig
	reader := bytes.NewReader(data)
	if err := json.NewDecoder(reader).Decode(&config); err != nil {
		return 0
	}

	fifoFd := int(data[0])
	_ = &linuxStandardInit{
		pipe:          pipe,
		consoleSocket: consoleSocket,
		parentPid:     unix.Getppid(),
		config:        config,
		fifoFd:        fifoFd,
	}
	return 1
}
