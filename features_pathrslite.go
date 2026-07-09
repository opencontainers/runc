//go:build !libpathrs

package main

func pathrsVersionString() string {
	return ""
}

func checkPathrsVersion() bool {
	return true
}
