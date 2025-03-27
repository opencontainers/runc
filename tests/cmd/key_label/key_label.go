package main

import (
	"log"
	"strings"

	"golang.org/x/sys/unix"
)

// This is a simple program to print the current session keyring name and its
// security label, to be run inside container (see selinux.bats). Can be
// thought of poor man's keyctl. Written in Go so we can have a static binary
// (a program in C would require libkeyutils which is usually provided only as
// a dynamic library).
func main() {
	id, err := unix.KeyctlGetKeyringID(unix.KEY_SPEC_SESSION_KEYRING, false)
	if err != nil {
		log.Fatalf("GetKeyringID: %v", err)
	}

	desc, err := unix.KeyctlString(unix.KEYCTL_DESCRIBE, id)
	if err != nil {
		log.Fatalf("KeyctlDescribe: %v", err)
	}
	// keyring;1000;1000;3f030000;_ses
	name := desc[strings.LastIndexByte(desc, ';')+1:]

	label, err := unix.KeyctlString(unix.KEYCTL_GET_SECURITY, id)
	if err != nil {
		log.Fatalf("KeyctlGetSecurity: %v", err)
	}

	println(name, label)
}
