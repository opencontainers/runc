package stacktrace

import "runtime"

// Capture captures a stacktrace for the current calling go program
//
// skip is the number of frames to skip
func Capture(userSkip int) Stacktrace {
	var (
		skip   = userSkip + 2 // add one for our own function, one for runtime.Callers
		frames []Frame
	)

	pc := make([]uintptr, 10)
	n := runtime.Callers(skip, pc)
	if n == 0 {
		return Stacktrace{}
	}
	f := runtime.CallersFrames(pc)
	for {
		frame, more := f.Next()
		frames = append(frames, newFrame(frame))
		if !more {
			break
		}
	}
	return Stacktrace{
		Frames: frames,
	}
}
