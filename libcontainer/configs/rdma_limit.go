package configs

type RdmaLimit struct {
	// A name of interface that supports RDMA.
	InterfaceName string `json:"interface_name"`

	// Limit of HCA Handle. -1 means max.
	HcaHandleLimit int64 `json:"hca_handle_limit"`

	// Limit of HCA Object. -1 means max.
	HcaObjectLimit int64 `json:"hca_object_limit"`
}
