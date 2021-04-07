// +build gofuzz

package libcontainer

import (
	"os"

	gofuzzheaders "github.com/AdaLogics/go-fuzz-headers"
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

	consoleSocket, err := os.OpenFile("consoleSocket.txt", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return -1
	}
	defer consoleSocket.Close()
	defer os.RemoveAll("consoleSocket.txt")

	// Create fuzzed initConfig
	config := new(initConfig)
	c := gofuzzheaders.NewConsumer(data)
	c.GenerateStruct(config)

	fifoFd := int(data[0])
	l := &linuxStandardInit{
		pipe:          pipe,
		consoleSocket: consoleSocket,
		parentPid:     unix.Getppid(),
		config:        config,
		fifoFd:        fifoFd,
	}
	_ = l.Init()
	return 1
}
