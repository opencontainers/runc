// Package logs provides helpers for logging used within runc (specifically for
// forwarding logs from "runc init" to the main runc process).
package logs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/sirupsen/logrus"
)

var fatalsSep = []byte("; ")

func ForwardLogs(logPipe io.ReadCloser) chan error {
	done := make(chan error, 1)
	s := bufio.NewScanner(logPipe)

	logger := logrus.StandardLogger()
	if logger.ReportCaller {
		// Need a copy of the standard logger, but with ReportCaller
		// turned off, as the logs are merely forwarded and their
		// true source is not this file/line/function.
		logNoCaller := *logrus.StandardLogger()
		logNoCaller.ReportCaller = false
		logger = &logNoCaller
	}

	go func() {
		fatals := []byte{}
		for s.Scan() {
			fatals = processEntry(s.Bytes(), logger, fatals)
		}
		if err := s.Err(); err != nil {
			logrus.Errorf("error reading from log source: %v", err)
		}
		if err := logPipe.Close(); err != nil {
			logrus.Errorf("error closing log source: %v", err)
		}
		// The only error we return is fatal messages from runc init.
		var err error
		if len(fatals) > 0 {
			err = errors.New(string(bytes.TrimSuffix(fatals, fatalsSep)))
		}
		done <- err
		close(done)
	}()

	return done
}

// processEntry parses the error and either logs it via the standard logger or,
// if this is a fatal error, appends its text to fatals.
func processEntry(text []byte, logger *logrus.Logger, fatals []byte) []byte {
	if len(text) == 0 {
		return fatals
	}

	var jl struct {
		Level logrus.Level `json:"level"`
		Msg   string       `json:"msg"`
	}
	if err := json.Unmarshal(text, &jl); err != nil {
		logrus.Errorf("failed to decode %q to json: %v", text, err)
		return fatals
	}

	if jl.Level == logrus.FatalLevel {
		fatals = append(fatals, jl.Msg...)
		fatals = append(fatals, fatalsSep...)
	} else {
		logger.Log(jl.Level, jl.Msg)
	}
	return fatals
}
