package libcontainer

import "io"

// API error code type.
type ErrorCode int

// API error codes.
const (
	// Factory errors
	IdInUse ErrorCode = iota
	InvalidIdFormat

	// Container errors
	ContainerNotExists
	ContainerPaused
	ContainerNotStopped

	// Common errors
	ConfigInvalid
	SystemError
)

func (c ErrorCode) String() string {
	switch c {
	case IdInUse:
		return "Id already in use"
	case InvalidIdFormat:
		return "Invalid format"
	case ContainerPaused:
		return "Container paused"
	case ConfigInvalid:
		return "Invalid configuration"
	case SystemError:
		return "System error"
	case ContainerNotExists:
		return "Container does not exist"
	case ContainerNotStopped:
		return "Container isn't stopped"
	default:
		return "Unknown error"
	}
}

// API Error type.
type Error interface {
	error

	// Returns a verbose string including the error message
	// and a representation of the stack trace suitable for
	// printing.
	Detail(w io.Writer) error

	// Returns the error code for this error.
	Code() ErrorCode
}

type initError struct {
	Message string `json:"message,omitempty"`
}

func (i initError) Error() string {
	return i.Message
}
