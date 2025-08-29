package configs

type LandlockConfig struct {
	Mode       string   `json:"mode"` // "enforce"|"best-effort"
	RoDirs     []string `json:"roDirs"`
	RwDirs     []string `json:"rwDirs"`
	WithRefer  []string `json:"withRefer"` // dirs that need cross-dir rename/link
	IoctlDev   []string `json:"ioctlDev"`  // device paths requiring ioctl
	BindTCP    []uint16 `json:"bindTCP"`
	ConnectTCP []uint16 `json:"connectTCP"`
}
