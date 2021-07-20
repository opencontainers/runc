// +build linux,cgo,seccomp

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	libseccomp "github.com/seccomp/libseccomp-golang"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	socketFile string
	pidFile    string
)

func init() {
	flag.StringVar(&socketFile, "socketfile", "/run/seccomp-agent.socket", "Socket file")
	flag.StringVar(&pidFile, "pid-file", "", "Pid file")
	logrus.SetLevel(logrus.DebugLevel)
}

func closeStateFds(recvFds []int) {
	for i := range recvFds {
		unix.Close(i)
	}
}

// parseStateFds returns the seccomp-fd and closes the rest of the fds in recvFds.
// In case of error, no fd is closed.
// StateFds is assumed to be formated as specs.ContainerProcessState.Fds and
// recvFds the corresponding list of received fds in the same SCM_RIGHT message.
func parseStateFds(stateFds []string, recvFds []int) (uintptr, error) {
	// Lets find the index in stateFds of the seccomp-fd.
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
		return 0, fmt.Errorf("seccomp fd not found or malformed containerProcessState.Fds")
	}

	if idx >= len(recvFds) || idx < 0 {
		return 0, fmt.Errorf("seccomp fd index out of range")
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
	MaxNameLen := 4096
	oobSpace := unix.CmsgSpace(4)
	stateBuf := make([]byte, 4096)
	oob := make([]byte, oobSpace)

	n, oobn, _, _, err := unix.Recvmsg(sockfd, stateBuf, oob, 0)
	if err != nil {
		return 0, "", err
	}
	if n >= MaxNameLen || oobn != oobSpace {
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
	if strings.HasPrefix(fileName, "/") {
		return unix.Mkdir(fmt.Sprintf("/proc/%d/root%s-%s", pid, fileName, metadata), mode)
	}

	return unix.Mkdir(fmt.Sprintf("/proc/%d/cwd/%s-%s", pid, fileName, metadata), mode)
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
		logrus.Debugf("Received syscall %q, pid %v, arch %q, args %+v\n", syscallName, req.Pid, req.Data.Arch, req.Data.Args)

		resp := &libseccomp.ScmpNotifResp{
			ID:    req.ID,
			Error: 0,
			Val:   0,
			Flags: libseccomp.NotifRespFlagContinue,
		}

		// TOCTOU check
		if err := libseccomp.NotifIDValid(fd, req.ID); err != nil {
			logrus.Errorf("TOCTOU check failed: req.ID is no longer valid: %s", err)
			resp.Error = int32(unix.ENOSYS)
			resp.Val = ^uint64(0) // -1
			goto sendResponse
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

			logrus.Debugf("mkdir: %q\n", fileName)

			err = runMkdirForContainer(req.Pid, fileName, uint32(req.Data.Args[1]), metadata)
			if err != nil {
				resp.Error = int32(unix.ENOSYS)
				resp.Val = ^uint64(0) // -1
			}
			resp.Flags = 0
		case "chmod":
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
	// Parse arguments
	flag.Parse()
	if flag.NArg() > 0 {
		flag.PrintDefaults()
		logrus.Fatal("Invalid command")
	}

	if err := os.RemoveAll(socketFile); err != nil {
		logrus.Fatalf("Cannot cleanup socket file %s: %v", socketFile, err)
	}

	if pidFile != "" {
		pid := fmt.Sprintf("%d\n", os.Getpid())
		if err := ioutil.WriteFile(pidFile, []byte(pid), 0o644); err != nil {
			logrus.Fatalf("Cannot write pid file %s: %v", pidFile, err)
		}
	}

	logrus.Info("Waiting for seccomp file descriptors")
	l, err := net.Listen("unix", socketFile)
	if err != nil {
		logrus.Fatalf("Cannot listen on %s: %s", socketFile, err)
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
			logrus.Errorf("Cannot get socket: %v\n", err)
			continue
		}
		newFd, metadata, err := handleNewMessage(int(socket.Fd()))
		socket.Close()
		if err != nil {
			logrus.Errorf("Error receiving seccomp file descriptor: %v", err)
			continue
		}
		logrus.Infof("Received new seccomp fd: %v\n", newFd)
		go notifHandler(libseccomp.ScmpFd(newFd), metadata)
	}
}
