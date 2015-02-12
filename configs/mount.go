package configs

type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`      // Source path, in the host namespace
	Destination string `json:"destination"` // Destination path, in the container
	Writable    bool   `json:"writable"`
	Relabel     string `json:"relabel"` // Relabel source if set, "z" indicates shared, "Z" indicates unshared
	Private     bool   `json:"private"`
	Slave       bool   `json:"slave"`
}
