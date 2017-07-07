package libcontainer

import (
	"os"
)

// newConsole returns an initialized console that can be used within a container
func newConsole() (Console, error) {
	return &windowsConsole{}, nil
}

// windowsConsole is a Windows pseudo TTY for use within a container.
type windowsConsole struct {
}

func (c *windowsConsole) Fd() uintptr {
	return 0
}

func (c *windowsConsole) File() *os.File {
	return nil
}

func (c *windowsConsole) Path() string {
	return ""
}

func (c *windowsConsole) Read(b []byte) (int, error) {
	return 0, nil
}

func (c *windowsConsole) Write(b []byte) (int, error) {
	return 0, nil
}

func (c *windowsConsole) Close() error {
	return nil
}
