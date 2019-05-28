// Package api provides a general purpose API which matches the main command
// line interface
package api

// Runc specifies the public interface of the public API
type Runc interface {
	// Version returns the Runc Version instance
	Version() *Version

	// WithDebug configures Runc to enable or disable debug output
	WithDebug(bool) Runc

	// WithRoot configures Runc to set the runtime root directory
	WithRoot(string) Runc

	// instance returns the internally used private runc type
	instance() *runc
}

// runc is the internal state type
type runc struct {
	root  string
	debug bool
}

// NewRunc creats a new instance of the Runc interface
func New() Runc {
	return &runc{
		root:  "/run/runc",
		debug: false,
	}
}

func (r *runc) instance() *runc {
	return r
}

func (r *runc) WithRoot(root string) Runc {
	r.instance().root = root
	return r
}

func (r *runc) WithDebug(debug bool) Runc {
	r.instance().debug = debug
	return r
}
