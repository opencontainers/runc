package integration

import (
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer"
)

func BenchmarkExecTrue(b *testing.B) {
	config := newTemplateConfig(b, nil)
	container, err := newContainer(b, config)
	ok(b, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(b, err)
	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer func() {
		_ = stdinW.Close()
		if _, err := process.Wait(); err != nil {
			b.Log(err)
		}
	}()
	ok(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exec := &libcontainer.Process{
			Cwd:      "/",
			Args:     []string{"/bin/true"},
			Env:      standardEnvironment,
			LogLevel: "0", // Minimize forwardChildLogs involvement.
		}
		err := container.Run(exec)
		if err != nil {
			b.Fatal("exec failed:", err)
		}
		waitProcess(exec, b)
	}
	b.StopTimer()
}
