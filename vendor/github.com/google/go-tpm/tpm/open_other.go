//go:build !windows

// Copyright (c) 2019, Google LLC All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tpm

import (
	"fmt"
	"io"

	"github.com/google/go-tpm/tpmutil"
)

// OpenTPM opens a channel to the TPM at the given path. If the file is a
// device, then it treats it like a normal TPM device, and if the file is a
// Unix domain socket, then it opens a connection to the socket.
func OpenTPM(path string) (io.ReadWriteCloser, error) {
	return openAndStartupTPM(path, false)
}

// openAndStartupTPM opens the TPM and optionally runs TPM_Startup if needed.
// This feature is implemented only for testing.
func openAndStartupTPM(path string, doStartup bool) (io.ReadWriteCloser, error) {
	rwc, err := tpmutil.OpenTPM(path)
	if err != nil {
		return nil, err
	}

	// Make sure this is a TPM 1.2
	_, err = GetManufacturer(rwc)
	if doStartup && err == tpmError(errInvalidPostInit) {
		if err = startup(rwc); err == nil {
			_, err = GetManufacturer(rwc)
		}
	}
	if err != nil {
		rwc.Close()
		return nil, fmt.Errorf("open %s: device is not a TPM 1.2: %v", path, err)
	}
	return rwc, nil
}
