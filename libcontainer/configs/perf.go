package configs

type PerfEvent struct {
	Config int `json:"config"`
	Ext1   int `json:"ext1"`
	Ext2   int `json:"ext2"`
}

type PerfGroup struct {
	Events []PerfEvent `json:"events"`
}
