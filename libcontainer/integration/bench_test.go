package integration

import (
	"bytes"
	"math/rand"
	"os"
	"strings"
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
		waitProcess(process, b)
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
		ok(b, err)
		waitProcess(exec, b)
	}
	b.StopTimer()
}

func genBigEnv(count int) []string {
	randStr := func(length int) string {
		const charset = "abcdefghijklmnopqrstuvwxyz0123456789_"
		b := make([]byte, length)
		for i := range b {
			b[i] = charset[rand.Intn(len(charset))]
		}
		return string(b)
	}

	envs := make([]string, count)
	for i := range count {
		key := strings.ToUpper(randStr(10))
		value := randStr(20)
		envs[i] = key + "=" + value
	}

	return envs
}

func BenchmarkExecInBigEnv(b *testing.B) {
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
		waitProcess(process, b)
	}()
	ok(b, err)

	const numEnv = 5000
	env := append(standardEnvironment, genBigEnv(numEnv)...)
	// Construct the expected output.
	var wantOut bytes.Buffer
	for _, e := range env {
		wantOut.WriteString(e + "\n")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffers := newStdBuffers()
		exec := &libcontainer.Process{
			Cwd:    "/",
			Args:   []string{"env"},
			Env:    env,
			Stdin:  buffers.Stdin,
			Stdout: buffers.Stdout,
			Stderr: buffers.Stderr,
		}
		err = container.Run(exec)
		ok(b, err)
		waitProcess(exec, b)
		if !bytes.Equal(buffers.Stdout.Bytes(), wantOut.Bytes()) {
			b.Fatalf("unexpected output: %s (stderr: %s)", buffers.Stdout, buffers.Stderr)
		}
	}
	b.StopTimer()
}
