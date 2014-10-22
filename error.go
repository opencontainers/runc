package libcontainer

// API error code type.
type ErrorCode int

// API error codes.
const (
	// Factory errors
	IdInUse ErrorCode = iota
	InvalidIdFormat

	// Container errors
	ContainerDestroyed
	ContainerPaused

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
	case ContainerDestroyed:
		return "Container destroyed"
	case ContainerPaused:
		return "Container paused"
	case ConfigInvalid:
		return "Invalid configuration"
	case SystemError:
		return "System Error"
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
	Detail() string

	// Returns the error code for this error.
	Code() ErrorCode
}
