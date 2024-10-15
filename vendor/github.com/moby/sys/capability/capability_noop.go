// Copyright 2023 The Capability Authors.
// Copyright 2013 Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !linux

package capability

import "errors"

var errNotSup = errors.New("not supported")

func newPid(_ int) (Capabilities, error) {
	return nil, errNotSup
}

func newFile(_ string) (Capabilities, error) {
	return nil, errNotSup
}

func lastCap() (Cap, error) {
	return -1, errNotSup
}

func ambientRaise(cap ...Cap) error {
	return errNotSup
}

func ambientLower(cap ...Cap) error {
	return errNotSup
}

func ambientClearAll() error {
	return errNotSup
}
