package utils

import (
	"os"

	"github.com/opencontainers/runc/internal/cmsg"
)

// RecvFile waits for a file descriptor to be sent over the given AF_UNIX
// socket. The file name of the remote file descriptor will be recreated
// locally (it is sent as non-auxiliary data in the same payload).
//
// Deprecated: This method is deprecated and has been moved to an internal
// package (see [cmsg.RecvFile]). It will be removed in runc 1.6.
func RecvFile(socket *os.File) (*os.File, error) {
	return cmsg.RecvFile(socket)
}

// SendFile sends a file over the given AF_UNIX socket. file.Name() is also
// included so that if the other end uses RecvFile, the file will have the same
// name information.
//
// Deprecated: This method is deprecated and has been moved to an internal
// package (see [cmsg.SendFile]). It will be removed in runc 1.6.
func SendFile(socket, file *os.File) error {
	return cmsg.SendFile(socket, file)
}

// SendRawFd sends a specific file descriptor over the given AF_UNIX socket.
//
// Deprecated: This method is deprecated and has been moved to an internal
// package (see [cmsg.SendRawFd]). It will be removed in runc 1.6.
func SendRawFd(socket *os.File, msg string, fd uintptr) error {
	return cmsg.SendRawFd(socket, msg, fd)
}
