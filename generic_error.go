package libcontainer

import (
	"fmt"
	"io"
	"text/template"
	"time"

	"github.com/docker/libcontainer/stacktrace"
)

var errorTemplate = template.Must(template.New("error").Parse(`Timestamp: {{.Timestamp}}
Code: {{.ECode}}
Message: {{.Err.Error}}
Frames:{{range $i, $frame := .Stack.Frames}}
---
{{$i}}: {{$frame.Function}}
Package: {{$frame.Package}}
File: {{$frame.File}}{{end}}
`))

func newGenericError(err error, c ErrorCode) Error {
	if le, ok := err.(Error); ok {
		return le
	}
	return &genericError{
		Timestamp: time.Now(),
		Err:       err,
		ECode:     c,
		Stack:     stacktrace.Capture(2),
	}
}

func newSystemError(err error) Error {
	if le, ok := err.(Error); ok {
		return le
	}
	return &genericError{
		Timestamp: time.Now(),
		Err:       err,
		ECode:     SystemError,
		Stack:     stacktrace.Capture(2),
	}
}

type genericError struct {
	Timestamp time.Time
	ECode     ErrorCode
	Err       error
	Stack     stacktrace.Stacktrace
}

func (e *genericError) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e.ECode, e.ECode, e.Err)
}

func (e *genericError) Code() ErrorCode {
	return e.ECode
}

func (e *genericError) Detail(w io.Writer) error {
	return errorTemplate.Execute(w, e)
}
