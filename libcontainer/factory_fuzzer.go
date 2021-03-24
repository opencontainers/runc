package libcontainer

import (
	"errors"
	"os"

	gofuzzheaders "github.com/AdaLogics/go-fuzz-headers"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/runc/libcontainer/configs/validate"
)

func createFiles(files []string, cf *gofuzzheaders.ConsumeFuzzer) error {
	for i := 0; i < len(files); i++ {
		f, err := os.OpenFile(files[i], os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return errors.New("Could not create file")
		}
		defer f.Close()
		b, err := cf.GetBytes()
		if err != nil {
			return errors.New("Could not get bytes")
		}
		_, err = f.Write(b)
		if err != nil {
			return errors.New("Could not write to file")
		}
	}
	return nil
}

func FuzzFactory(data []byte) int {
	err := os.MkdirAll("/tmp/fuzz-root", 0777)
	if err != nil {
		return -1
	}
	err = os.MkdirAll("/tmp/fuzz-root/fuzz", 0777)
	if err != nil {
		return -1
	}
	factory, err := New("/tmp/fuzz-root")
	if err != nil {
		return -1
	}
	root := "/tmp/fuzz-root"
	factory = &LinuxFactory{
		Root:      root,
		InitPath:  "/proc/self/exe",
		InitArgs:  []string{os.Args[0], "init"},
		Validator: validate.New(),
		CriuPath:  "criu",
	}
	c := gofuzzheaders.NewConsumer(data)
	stateFilename := "state.json"
	state_json := []string{"/tmp/fuzz-root/fuzz/state.json"}
	err = createFiles(state_json, c)
	if err != nil {
		return -1
	}
	defer os.RemoveAll("/tmp/fuzz-root/state.json")

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
