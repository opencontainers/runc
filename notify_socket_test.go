package main

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestNotifyHost tests how runc reports container readiness to the host (usually systemd).
func TestNotifyHost(t *testing.T) {
	addr := net.UnixAddr{
		Name: t.TempDir() + "/testsocket",
		Net:  "unixgram",
	}

	server, err := net.ListenUnixgram("unixgram", &addr)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	client, err := net.DialUnix("unixgram", nil, &addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// run notifyHost in a separate goroutine
	notifyHostChan := make(chan error)
	go func() {
		notifyHostChan <- notifyHost(client, []byte("READY=42"), 1337)
	}()

	// mock a host process listening for runc's notifications
	expectRead(t, server, "READY=42\n")
	expectRead(t, server, "MAINPID=1337\n")
	expectBarrier(t, server, notifyHostChan)
}

func expectRead(t *testing.T, r io.Reader, expected string) {
	var buf [1024]byte
	n, err := r.Read(buf[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf[:n], []byte(expected)) {
		t.Fatalf("Expected to read '%s' but runc sent '%s' instead", expected, buf[:n])
	}
}

func expectBarrier(t *testing.T, conn *net.UnixConn, notifyHostChan <-chan error) {
	var msg, oob [1024]byte
	n, oobn, _, _, err := conn.ReadMsgUnix(msg[:], oob[:])
	if err != nil {
		t.Fatal("Failed to receive BARRIER message", err)
	}
	if !bytes.Equal(msg[:n], []byte("BARRIER=1")) {
		t.Fatalf("Expected to receive 'BARRIER=1' but got '%s' instead.", msg[:n])
	}

	fd := mustExtractFd(t, oob[:oobn])

	// Test whether notifyHost actually honors the barrier
	timer := time.NewTimer(500 * time.Millisecond)
	select {
	case <-timer.C:
		// this is the expected case
		break
	case <-notifyHostChan:
		t.Fatal("runc has terminated before barrier was lifted")
	}

	// Lift the barrier
	err = unix.Close(fd)
	if err != nil {
		t.Fatal(err)
	}

	// Expect notifyHost to terminate now
	err = <-notifyHostChan
	if err != nil {
		t.Fatal("notifyHost function returned with error", err)
	}
}

func mustExtractFd(t *testing.T, buf []byte) int {
	cmsgs, err := unix.ParseSocketControlMessage(buf)
	if err != nil {
		t.Fatal("Failed to parse control message", err)
	}

	fd := 0
	seenScmRights := false
	for _, cmsg := range cmsgs {
		if cmsg.Header.Type != unix.SCM_RIGHTS {
			continue
		}
		if seenScmRights {
			t.Fatal("Expected to see exactly one SCM_RIGHTS message, but got a second one")
		}
		seenScmRights = true
		fds, err := unix.ParseUnixRights(&cmsg)
		if err != nil {
			t.Fatal("Failed to parse SCM_RIGHTS message", err)
		}
		if len(fds) != 1 {
			t.Fatal("Expected to read exactly one file descriptor, but got", len(fds))
		}
		fd = fds[0]
	}
	if !seenScmRights {
		t.Fatal("Control messages didn't contain an SCM_RIGHTS message")
	}

	return fd
}
