package libcontainer

import (
	"bytes"
	"fmt"
	"runtime"
	"time"
)

var newLine = []byte("\n")

func newGenericError(err error, c ErrorCode) Error {
	return &GenericError{
		timestamp: time.Now(),
		err:       err,
		code:      c,
		stack:     captureStackTrace(2),
	}
}

func captureStackTrace(skip int) string {
	buf := make([]byte, 4096)
	buf = buf[:runtime.Stack(buf, true)]

	lines := bytes.Split(buf, newLine)
	return string(bytes.Join(lines[skip:], newLine))
}

type GenericError struct {
	timestamp time.Time
	code      ErrorCode
	err       error
	stack     string
}

func (e *GenericError) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e.code, e.code, e.err)
}

func (e *GenericError) Code() ErrorCode {
	return e.code
}

func (e *GenericError) Detail() string {
	return fmt.Sprintf("[%d] %s\n%s", e.code, e.err, e.stack)
}
