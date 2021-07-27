package logs

import (
	"bufio"
	"encoding/json"
	"io"

	"github.com/sirupsen/logrus"
)

func ForwardLogs(logPipe io.ReadCloser) chan error {
	done := make(chan error, 1)
	s := bufio.NewScanner(logPipe)

	go func() {
		for s.Scan() {
			processEntry(s.Bytes())
		}
		if err := logPipe.Close(); err != nil {
			logrus.Errorf("error closing log source: %v", err)
		}
		// The only error we want to return is when reading from
		// logPipe has failed.
		done <- s.Err()
		close(done)
	}()

	return done
}

func processEntry(text []byte) {
	if len(text) == 0 {
		return
	}

	var jl struct {
		Level logrus.Level `json:"level"`
		Msg   string       `json:"msg"`
	}
	if err := json.Unmarshal(text, &jl); err != nil {
		logrus.Errorf("failed to decode %q to json: %v", text, err)
		return
	}

	logrus.StandardLogger().Logf(jl.Level, jl.Msg)
}
