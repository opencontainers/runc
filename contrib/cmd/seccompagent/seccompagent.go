//go:build linux && seccomp
// +build linux,seccomp

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/runtime-spec/specs-go"
	libseccomp "github.com/seccomp/libseccomp-golang"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	socketFile string
	pidFile    string
)

func closeStateFds(recvFds []int) {
	for i := range recvFds {
		unix.Close(i)
	}
}

// parseStateFds returns the seccomp-fd and closes the rest of the fds in recvFds.
// In case of error, no fd is closed.
// StateFds is assumed to be formatted as specs.ContainerProcessState.Fds and
// recvFds the corresponding list of received fds in the same SCM_RIGHT message.
func parseStateFds(stateFds []string, recvFds []int) (uintptr, error) {
	// Let's find the index in stateFds of the seccomp-fd.
	idx := -1
	err := false

	for i, name := range stateFds {
		if name == specs.SeccompFdName && idx == -1 {
			idx = i
			continue
		}

		// We found the seccompFdName twice. Error out!
		if name == specs.SeccompFdName && idx != -1 {
			err = true
		}
	}

	if idx == -1 || err {
		return 0, errors.New("seccomp fd not found or malformed containerProcessState.Fds")
	}

	if idx >= len(recvFds) || idx < 0 {
		return 0, errors.New("seccomp fd index out of range")
	}

	fd := uintptr(recvFds[idx])

	for i := range recvFds {
		if i == idx {
			continue
		}

		unix.Close(recvFds[i])
	}

	return fd, nil
}

func handleNewMessage(sockfd int) (uintptr, string, error) {
	const maxNameLen = 4096
	stateBuf := make([]byte, maxNameLen)
	oobSpace := unix.CmsgSpace(4)
	oob := make([]byte, oobSpace)

	n, oobn, _, _, err := unix.Recvmsg(sockfd, stateBuf, oob, 0)
	if err != nil {
		return 0, "", err
	}
	if n >= maxNameLen || oobn != oobSpace {
		return 0, "", fmt.Errorf("recvfd: incorrect number of bytes read (n=%d oobn=%d)", n, oobn)
	}

	// Truncate.
	stateBuf = stateBuf[:n]
	oob = oob[:oobn]

	scms, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return 0, "", err
	}
	if len(scms) != 1 {
		return 0, "", fmt.Errorf("recvfd: number of SCMs is not 1: %d", len(scms))
	}
	scm := scms[0]

	fds, err := unix.ParseUnixRights(&scm)
	if err != nil {
		return 0, "", err
	}

	containerProcessState := &specs.ContainerProcessState{}
	err = json.Unmarshal(stateBuf, containerProcessState)
	if err != nil {
		closeStateFds(fds)
		return 0, "", fmt.Errorf("cannot parse OCI state: %w", err)
	}

	fd, err := parseStateFds(containerProcessState.Fds, fds)
	if err != nil {
		closeStateFds(fds)
		return 0, "", err
	}

	return fd, containerProcessState.Metadata, nil
}

func readArgString(pid uint32, offset int64) (string, error) {
	buffer := make([]byte, 4096) // PATH_MAX

	memfd, err := unix.Open(fmt.Sprintf("/proc/%d/mem", pid), unix.O_RDONLY, 0o777)
	if err != nil {
		return "", err
	}
	defer unix.Close(memfd)

	_, err = unix.Pread(memfd, buffer, offset)
	if err != nil {
		return "", err
	}

	buffer[len(buffer)-1] = 0
	s := buffer[:bytes.IndexByte(buffer, 0)]
	return string(s), nil
}

func runMkdirForContainer(pid uint32, fileName string, mode uint32, metadata string) error {
	// We validated before that metadata is not a string that can make
	// newFile a file in a different location other than root.
	newFile := fmt.Sprintf("%s-%s", fileName, metadata)
	root := fmt.Sprintf("/proc/%d/cwd/", pid)

	if strings.HasPrefix(fileName, "/") {
		// If it starts with /, use the rootfs as base
		root = fmt.Sprintf("/proc/%d/root/", pid)
	}

	path, err := securejoin.SecureJoin(root, newFile)
	if err != nil {
		return err
	}

	return unix.Mkdir(path, mode)
}

// notifHandler handles seccomp notifications and responses
func notifHandler(fd libseccomp.ScmpFd, metadata string) {
	defer unix.Close(int(fd))
	for {
		req, err := libseccomp.NotifReceive(fd)
		if err != nil {
			logrus.Errorf("Error in NotifReceive(): %s", err)
			continue
		}
		syscallName, err := req.Data.Syscall.GetName()
		if err != nil {
			logrus.Errorf("Error decoding syscall %v(): %s", req.Data.Syscall, err)
			continue
		}
		logrus.Debugf("Received syscall %q, pid %v, arch %q, args %+v", syscallName, req.Pid, req.Data.Arch, req.Data.Args)

		resp := &libseccomp.ScmpNotifResp{
			ID:    req.ID,
			Error: 0,
			Val:   0,
			Flags: libseccomp.NotifRespFlagContinue,
		}

		// TOCTOU check
		if err := libseccomp.NotifIDValid(fd, req.ID); err != nil {
			logrus.Errorf("TOCTOU check failed: req.ID is no longer valid: %s", err)
			continue
		}

		switch syscallName {
		case "mkdir":
			fileName, err := readArgString(req.Pid, int64(req.Data.Args[0]))
			if err != nil {
				logrus.Errorf("Cannot read argument: %s", err)
				resp.Error = int32(unix.ENOSYS)
				resp.Val = ^uint64(0) // -1
				goto sendResponse
			}

			logrus.Debugf("mkdir: %q", fileName)

			// TOCTOU check
			if err := libseccomp.NotifIDValid(fd, req.ID); err != nil {
				logrus.Errorf("TOCTOU check failed: req.ID is no longer valid: %s", err)
				continue
			}

			err = runMkdirForContainer(req.Pid, fileName, uint32(req.Data.Args[1]), metadata)
			if err != nil {
				resp.Error = int32(unix.ENOSYS)
				resp.Val = ^uint64(0) // -1
			}
			resp.Flags = 0
		case "chmod", "fchmod", "fchmodat":
			resp.Error = int32(unix.ENOMEDIUM)
			resp.Val = ^uint64(0) // -1
			resp.Flags = 0
		}

	sendResponse:
		if err = libseccomp.NotifRespond(fd, resp); err != nil {
			logrus.Errorf("Error in notification response: %s", err)
			continue
		}
	}
}

func main() {
	flag.StringVar(&socketFile, "socketfile", "/run/seccomp-agent.socket", "Socket file")
	flag.StringVar(&pidFile, "pid-file", "", "Pid file")
	logrus.SetLevel(logrus.DebugLevel)

	// Parse arguments
	flag.Parse()
	if flag.NArg() > 0 {
		flag.PrintDefaults()
		logrus.Fatal("Invalid command")
	}

	if err := os.Remove(socketFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		logrus.Fatalf("Cannot cleanup socket file: %v", err)
	}

	if pidFile != "" {
		pid := fmt.Sprintf("%d", os.Getpid())
		if err := os.WriteFile(pidFile, []byte(pid), 0o644); err != nil {
			logrus.Fatalf("Cannot write pid file: %v", err)
		}
	}

	logrus.Info("Waiting for seccomp file descriptors")
	l, err := net.Listen("unix", socketFile)
	if err != nil {
		logrus.Fatalf("Cannot listen: %s", err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			logrus.Errorf("Cannot accept connection: %s", err)
			continue
		}
		socket, err := conn.(*net.UnixConn).File()
		conn.Close()
		if err != nil {
			logrus.Errorf("Cannot get socket: %v", err)
			continue
		}
		newFd, metadata, err := handleNewMessage(int(socket.Fd()))
		socket.Close()
		if err != nil {
			logrus.Errorf("Error receiving seccomp file descriptor: %v", err)
			continue
		}

		// Make sure we don't allow strings like "/../p", as that means
		// a file in a different location than expected. We just want
		// safe things to use as a suffix for a file name.
		metadata = filepath.Base(metadata)
		if strings.Contains(metadata, "/") {
			// Fallback to a safe string.
			metadata = "agent-generated-suffix"
		}

		logrus.Infof("Received new seccomp fd: %v", newFd)
		go notifHandler(libseccomp.ScmpFd(newFd), metadata)
	}
}
