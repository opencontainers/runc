// getfattr is a simple program to print a single extended attribute of a file,
// to be run inside container (see selinux.bats). It is a poor man's getfattr
// because the test images do not ship attr(1). Written in Go so we can have a
// static binary.
//
// This tool is only intended to be used within runc's integration tests.
package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/sys/unix"
)

func getxattr(path, name string) (string, error) {
	size, err := unix.Getxattr(path, name, nil)
	if err != nil {
		return "", err
	}
	// The buffer size may be outdated if the attribute was changed in the
	// meantime, so ask for the value again.
	buf := make([]byte, size)
	size, err = unix.Getxattr(path, name, buf)
	if err != nil {
		return "", err
	}
	// The SELinux context is stored with a trailing NUL byte, and some
	// filesystems may not report its length at all.
	for size > 0 && buf[size-1] == 0 {
		size--
	}
	return string(buf[:size]), nil
}

func main() {
	if len(os.Args) != 4 || os.Args[1] != "-n" {
		log.Fatalf("Usage: %s -n NAME FILE", os.Args[0])
	}
	name, path := os.Args[2], os.Args[3]

	value, err := getxattr(path, name)
	if err != nil {
		log.Fatalf("getfattr: %s: %v", path, err)
	}
	// Mimic the output format of attr(1)'s getfattr.
	fmt.Printf("# file: %s\n%s=\"%s\"\n", path, name, value)
}
