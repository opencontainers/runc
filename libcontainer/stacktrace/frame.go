package stacktrace

import (
	"path/filepath"
	"runtime"
	"strings"
)

func newFrame(frame runtime.Frame) Frame {
	pack, name := parseFunctionName(frame.Function)
	return Frame{
		File:     filepath.Base(frame.File),
		Function: name,
		Package:  pack,
		Line:     frame.Line,
	}
}

func parseFunctionName(name string) (string, string) {
	i := strings.LastIndex(name, ".")
	if i == -1 {
		return "", name
	}
	return name[:i], name[i+1:]
}

// Frame contains all the information for a stack frame within a go program
type Frame struct {
	File     string
	Function string
	Package  string
	Line     int
}
