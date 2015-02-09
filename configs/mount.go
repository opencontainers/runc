package configs

type Mount struct {
	Type        string `json:"type,omitempty"`
	Source      string `json:"source,omitempty"`      // Source path, in the host namespace
	Destination string `json:"destination,omitempty"` // Destination path, in the container
	Writable    bool   `json:"writable,omitempty"`
	Relabel     string `json:"relabel,omitempty"` // Relabel source if set, "z" indicates shared, "Z" indicates unshared
	Private     bool   `json:"private,omitempty"`
	Slave       bool   `json:"slave,omitempty"`
}
