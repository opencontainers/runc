package logs

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

const msgErr = `"level":"error"`

func TestLoggingToFile(t *testing.T) {
	l := runLogForwarding(t)

	msg := `"level":"info","msg":"kitten"`
	logToLogWriter(t, l, msg)
	finish(t, l)
	check(t, l, msg, msgErr)
}

func TestLogForwardingDoesNotStopOnJsonDecodeErr(t *testing.T) {
	l := runLogForwarding(t)

	logToLogWriter(t, l, `"invalid-json-with-kitten"`)
	checkWait(t, l, msgErr, "")

	truncateLogFile(t, l.file)

	msg := `"level":"info","msg":"puppy"`
	logToLogWriter(t, l, msg)
	finish(t, l)
	check(t, l, msg, msgErr)
}

func TestLogForwardingDoesNotStopOnLogLevelParsingErr(t *testing.T) {
	l := runLogForwarding(t)

	msg := `"level":"alert","msg":"puppy"`
	logToLogWriter(t, l, msg)
	checkWait(t, l, msgErr, msg)

	truncateLogFile(t, l.file)

	msg = `"level":"info","msg":"puppy"`
	logToLogWriter(t, l, msg)
	finish(t, l)
	check(t, l, msg, msgErr)
}

func TestLogForwardingStopsAfterClosingTheWriter(t *testing.T) {
	l := runLogForwarding(t)

	msg := `"level":"info","msg":"sync"`
	logToLogWriter(t, l, msg)

	// Do not use finish() here as we check done pipe ourselves.
	l.w.Close()
	select {
	case <-l.done:
	case <-time.After(10 * time.Second):
		t.Fatal("log forwarding did not stop after closing the pipe")
	}

	check(t, l, msg, msgErr)
}

func logToLogWriter(t *testing.T, l *log, message string) {
	t.Helper()
	_, err := l.w.Write([]byte("{" + message + "}\n"))
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

	tempFile, err := os.CreateTemp("", "")
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

// check checks that the file contains txt and does not contain notxt.
func check(t *testing.T, l *log, txt, notxt string) {
	t.Helper()
	contents, err := os.ReadFile(l.file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if txt != "" && !bytes.Contains(contents, []byte(txt)) {
		t.Fatalf("%s does not contain %s", contents, txt)
	}
	if notxt != "" && bytes.Contains(contents, []byte(notxt)) {
		t.Fatalf("%s does contain %s", contents, notxt)
	}
}

// checkWait is like check, but if the file is empty,
// it waits until it's not.
func checkWait(t *testing.T, l *log, txt string, notxt string) {
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

	check(t, l, txt, notxt)
}
