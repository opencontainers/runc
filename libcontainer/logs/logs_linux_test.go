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

	logToLogWriter(t, l, `{"level": "info","msg":"kitten"}`)
	finish(t, l)
	check(t, l, "kitten")
}

func TestLogForwardingDoesNotStopOnJsonDecodeErr(t *testing.T) {
	l := runLogForwarding(t)

	logToLogWriter(t, l, "invalid-json-with-kitten")
	checkWait(t, l, "failed to decode")

	truncateLogFile(t, l.file)

	logToLogWriter(t, l, `{"level": "info","msg":"puppy"}`)
	finish(t, l)
	check(t, l, "puppy")
}

func TestLogForwardingDoesNotStopOnLogLevelParsingErr(t *testing.T) {
	l := runLogForwarding(t)

	logToLogWriter(t, l, `{"level": "alert","msg":"puppy"}`)
	checkWait(t, l, "failed to parse log level")

	truncateLogFile(t, l.file)

	logToLogWriter(t, l, `{"level": "info","msg":"puppy"}`)
	finish(t, l)
	check(t, l, "puppy")
}

func TestLogForwardingStopsAfterClosingTheWriter(t *testing.T) {
	l := runLogForwarding(t)

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
	file *os.File
	done chan error
}

func runLogForwarding(t *testing.T) *log {
	t.Helper()
	logR, logW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		logR.Close()
		logW.Close()
	})

	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	})

	logrus.SetOutput(tempFile)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	doneForwarding := ForwardLogs(logR)

	return &log{w: logW, done: doneForwarding, file: tempFile}
}

func finish(t *testing.T, l *log) {
	t.Helper()
	l.w.Close()
	if err := <-l.done; err != nil {
		t.Fatalf("ForwardLogs: %v", err)
	}
}

func truncateLogFile(t *testing.T, file *os.File) {
	t.Helper()

	err := file.Truncate(0)
	if err != nil {
		t.Fatalf("failed to truncate log file: %v", err)
	}
}

// check checks that file contains txt
func check(t *testing.T, l *log, txt string) {
	t.Helper()
	contents, err := ioutil.ReadFile(l.file.Name())
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
		st, err := l.file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if st.Size() > 0 {
			break
		}
		if i == iter {
			t.Fatalf("waited %s for file %s to be non-empty but it still is", iter*delay, l.file.Name())
		}
		time.Sleep(delay)
	}

	check(t, l, txt)
}
