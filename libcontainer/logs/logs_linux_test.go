package logs

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestLoggingToFile(t *testing.T) {
	l := runLogForwarding(t)
	defer l.cleanup()

	logToLogWriter(t, l, `{"level": "info","msg":"kitten"}`)
	finish(t, l)
	check(t, l, "kitten")
}

func TestLogForwardingDoesNotStopOnJsonDecodeErr(t *testing.T) {
	l := runLogForwarding(t)
	defer l.cleanup()

	logToLogWriter(t, l, "invalid-json-with-kitten")
	checkWait(t, l, "failed to decode")

	truncateLogFile(t, l.file)

	logToLogWriter(t, l, `{"level": "info","msg":"puppy"}`)
	finish(t, l)
	check(t, l, "puppy")
}

func TestLogForwardingDoesNotStopOnLogLevelParsingErr(t *testing.T) {
	l := runLogForwarding(t)
	defer l.cleanup()

	logToLogWriter(t, l, `{"level": "alert","msg":"puppy"}`)
	checkWait(t, l, "failed to parse log level")

	truncateLogFile(t, l.file)

	logToLogWriter(t, l, `{"level": "info","msg":"puppy"}`)
	finish(t, l)
	check(t, l, "puppy")
}

func TestLogForwardingStopsAfterClosingTheWriter(t *testing.T) {
	l := runLogForwarding(t)
	defer l.cleanup()

	logToLogWriter(t, l, `{"level": "info","msg":"sync"}`)

	// Do not use finish() here as we check done pipe ourselves.
	l.w.Close()
	select {
	case <-l.done:
	case <-time.After(10 * time.Second):
		t.Fatal("log forwarding did not stop after closing the pipe")
	}

	check(t, l, "sync")
}

func logToLogWriter(t *testing.T, l *log, message string) {
	t.Helper()
	_, err := l.w.Write([]byte(message + "\n"))
	if err != nil {
		t.Fatalf("failed to write %q to log writer: %v", message, err)
	}
}

type log struct {
	w    io.WriteCloser
	file string
	done chan error

	// TODO: use t.Cleanup after dropping support for Go 1.13
	cleanup func()
}

func runLogForwarding(t *testing.T) *log {
	t.Helper()
	logR, logW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	tempFile.Close()
	logFile := tempFile.Name()

	logConfig := Config{LogLevel: logrus.InfoLevel, LogFormat: "json", LogFilePath: logFile}
	loggingConfigured = false
	if err := ConfigureLogging(logConfig); err != nil {
		t.Fatal(err)
	}

	doneForwarding := ForwardLogs(logR)

	cleanup := func() {
		os.Remove(logFile)
		logR.Close()
		logW.Close()
	}

	return &log{w: logW, done: doneForwarding, file: logFile, cleanup: cleanup}
}

func finish(t *testing.T, l *log) {
	t.Helper()
	l.w.Close()
	if err := <-l.done; err != nil {
		t.Fatalf("ForwardLogs: %v", err)
	}
}

func truncateLogFile(t *testing.T, logFile string) {
	t.Helper()
	file, err := os.OpenFile(logFile, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
		return
	}
	defer file.Close()

	err = file.Truncate(0)
	if err != nil {
		t.Fatalf("failed to truncate log file: %v", err)
	}
}

// check checks that file contains txt
func check(t *testing.T, l *log, txt string) {
	t.Helper()
	contents, err := ioutil.ReadFile(l.file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contents, []byte(txt)) {
		t.Fatalf("%q does not contain %q", string(contents), txt)
	}
}

// checkWait checks that file contains txt. If the file is empty,
// it waits until it's not.
func checkWait(t *testing.T, l *log, txt string) {
	t.Helper()
	const (
		delay = 100 * time.Millisecond
		iter  = 3
	)
	for i := 0; ; i++ {
		st, err := os.Stat(l.file)
		if err != nil {
			t.Fatal(err)
		}
		if st.Size() > 0 {
			break
		}
		if i == iter {
			t.Fatalf("waited %s for file %s to be non-empty but it still is", iter*delay, l.file)
		}
		time.Sleep(delay)
	}

	check(t, l, txt)
}
