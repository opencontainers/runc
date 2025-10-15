package configs

type IntelRdt struct {
	// The identity for RDT Class of Service
	ClosID string `json:"closID,omitempty"`

	// Schemata is a generic field to specify schemata file in the resctrl
	// filesystem. Each element represents one line written to the schemata file.
	Schemata []string `json:"schemata,omitempty"`

	// The schema for L3 cache id and capacity bitmask (CBM)
	// Format: "L3:<cache_id0>=<cbm0>;<cache_id1>=<cbm1>;..."
	L3CacheSchema string `json:"l3_cache_schema,omitempty"`

	// The schema of memory bandwidth per L3 cache id
	// Format: "MB:<cache_id0>=bandwidth0;<cache_id1>=bandwidth1;..."
	// The unit of memory bandwidth is specified in "percentages" by
	// default, and in "MBps" if MBA Software Controller is enabled.
	MemBwSchema string `json:"memBwSchema,omitempty"`

	// Create a monitoring group for the container.
	EnableMonitoring bool `json:"enableMonitoring,omitempty"`
}
