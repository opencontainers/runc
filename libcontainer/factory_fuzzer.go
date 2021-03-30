package libcontainer

import (
	"encoding/json"
	"errors"
	"os"

	gofuzzheaders "github.com/AdaLogics/go-fuzz-headers"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/runc/libcontainer/configs/validate"
)

func createFiles(files []string, b []byte) error {
	for i := 0; i < len(files); i++ {
		f, err := os.OpenFile(files[i], os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return errors.New("Could not create file")
		}
		defer f.Close()
		_, err = f.Write(b)
		if err != nil {
			return errors.New("Could not write to file")
		}
	}
	return nil
}

type FuzzState struct {
	OciVersion  string   `json:"ociVersion"`
	Id          string   `json:"id"`
	Status      string   `json:"status"`
	Pid         int      `json:"pid"`
	Bundle      string   `json:"bundle"`
	Annotations []string `json:"annotations"`
}

func FuzzFactory(data []byte) int {
	if len(data) < 20 {
		return -1
	}
	root := "/tmp/fuzz-root"
	err := os.MkdirAll(root, 0777)
	if err != nil {
		return -1
	}
	err = os.MkdirAll("/tmp/fuzz-root/fuzz", 0777)
	if err != nil {
		return -1
	}
	factory := &LinuxFactory{
		Root:      root,
		InitPath:  "/proc/self/exe",
		InitArgs:  []string{os.Args[0], "init"},
		Validator: validate.New(),
		CriuPath:  "criu",
	}
	c := gofuzzheaders.NewConsumer(data)
	fs := FuzzState{}
	ociVersion, err := c.GetString()
	if err != nil {
		return 0
	}
	id, err := c.GetString()
	if err != nil {
		return 0
	}
	status, err := c.GetString()
	if err != nil {
		return 0
	}
	pid, err := c.GetInt()
	if err != nil {
		return 0
	}
	bundle, err := c.GetString()
	if err != nil {
		return 0
	}
	if len(ociVersion) < 5 || len(id) < 5 || len(bundle) < 5 {
		return 0
	}

	fs.OciVersion = ociVersion
	fs.Id = id
	fs.Status = status
	fs.Pid = pid
	fs.Bundle = bundle
	fs.Annotations = []string{"fuzz"}
	b, err := json.Marshal(&fs)
	if err != nil {
		return 0
	}

	stateFilename := "state.json"
	state_json_path := "/tmp/fuzz-root/fuzz/state.json"
	state_json := []string{state_json_path}
	err = createFiles(state_json, b)
	if err != nil {
		return 0
	}
	defer os.RemoveAll(state_json_path)

	stateFilePath, err := securejoin.SecureJoin("/tmp/fuzz-root/fuzz", stateFilename)
	if err != nil {
		return -1
	}
	f, err := os.Open(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return -1
		}
	}
	defer f.Close()

	_, _ = factory.Load("fuzz")
	return 1
}
