//go:build !linux || !seccomp
// +build !linux !seccomp

package main

import "fmt"

func main() {
	fmt.Println("Not supported, to use this compile with build tag: seccomp.")
}
