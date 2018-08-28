// +build linux

package main

import (
	"strings"
)

// refer to https://salsa.debian.org/debian/procps/blob/master/ps/parser.c
var (
	// bsdOptions list all ps params with short style
	// if with -, remove it to bsd style
	// the map value means:
	//    0 - Don't need append value
	//    1 - Need append value
	bsdOptions = map[rune]int{
		'a': 0,
		'A': 0,
		'd': 0,
		'e': 0,
		'g': 0,
		'N': 0,
		'T': 0,
		'r': 0,
		'x': 0,
		'C': 1,
		'G': 1,
		'p': 1,
		'q': 1,
		's': 1,
		't': 1,
		'U': 1,
		'c': 0,
		'f': 0,
		'F': 0,
		'j': 0,
		'l': 0,
		'M': 0,
		'O': 1,
		'o': 1,
		'S': 0,
		'u': 0,
		'v': 0,
		'X': 0,
		'y': 0,
		'Z': 0,
		'h': 0,
		'H': 0,
		'k': 1,
		'n': 0,
		'w': 0,
		'L': 0,
		'm': 0,
		'V': 0,
	}

	// gnuLongOptions list all ps params with long style
	// the map value have the same means to bsdOptions
	gnuLongOptions = map[string]int{
		"deselect":    0,
		"Group":       1,
		"group":       1,
		"pid":         1,
		"ppid":        1,
		"quick-pid":   1,
		"sid":         1,
		"tty":         1,
		"User":        1,
		"user":        1,
		"context":     0,
		"format":      1,
		"cols":        1,
		"columns":     1,
		"cumulative":  0,
		"forest":      0,
		"headers":     0,
		"header":      0,
		"heading":     0,
		"headings":    0,
		"lines":       1,
		"no-headers":  0,
		"no-header":   0,
		"no-heading":  0,
		"no-headings": 0,
		"noheader":    0,
		"noheaders":   0,
		"noheading":   0,
		"noheadings":  0,
		"rows":        1,
		"sort":        1,
		"width":       1,
		"help":        0,
		"info":        0,
		"version":     0,
	}
)

// lexerTokenState indicates the state when analysis ps args
type lexerTokenState int

const (
	_ lexerTokenState = iota
	param
	value
)

// bsdValueNeed check whether needs a value after the short style option in ps args
func bsdValueNeed(pname rune) int {
	if v, ok := bsdOptions[pname]; ok {
		return v
	}
	return -1
}

// gnuLongValueNeed check whether needs a value after the long style option in ps args
func gnuLongValueNeed(pname string) int {
	if v, ok := gnuLongOptions[pname]; ok {
		return v
	}
	return -1
}

// UnixPsParamLexer returns a map whose key is ps option
func UnixPsParamLexer(psArgs []string) map[string]int {
	params := make(map[string]int)
	tokenState := param
	for _, args := range psArgs {
		start := 0

		switch tokenState {
		case param:
			if strings.HasPrefix(args, "--") {
				pNames := strings.Split(args[2:], "=")
				if len(pNames[0]) > 0 {
					switch gnuLongValueNeed(pNames[0]) {
					case 0:
						params[pNames[0]] = 1
						tokenState = param
					case 1:
						params[pNames[0]] = 1
						if len(pNames) == 1 {
							tokenState = value
						} else {
							tokenState = param
						}
					case -1:
						tokenState = param
					}
				}
			} else {
				if strings.HasPrefix(args, "-") {
					start = 1
				}
			Loop:
				for _, t := range args[start:] {
					switch tokenState {
					case param:
						switch bsdValueNeed(t) {
						case 0:
							params[string(t)] = 1
							tokenState = param
						case 1:
							params[string(t)] = 1
							tokenState = value
						case -1:
							tokenState = param
						}
					case value:
						tokenState = param
						break Loop
					}
				}
			}
		case value:
			tokenState = param
		}
	}
	return params
}
