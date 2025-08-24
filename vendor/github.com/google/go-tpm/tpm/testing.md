# Testing TPM 1.2 Functionality

**TODO(https://github.com/google/go-tpm/issues/91):** Support for testing the TPM 1.2 stack against
a simulator is a work in progress. Today, it requires several manual steps.

## Overview

As TPM 1.2s are phased out of common developer devices, testing changes to the TPM 1.2 stack is
difficult without special hardware. To support development on the TPM 1.2 stack without special
hardware, a TPM 1.2 simulator or emulator may be used. This document discusses how to use
[IBM's TPM 1.2 simulator](http://ibmswtpm.sourceforge.net) (on a Linux or Mac OS device, Windows is
not yet supported) to run the go-tpm TPM 1.2 tests (in the `go-tpm/tpm/` directory).

## Downloading, building, and using the IBM TPM 1.2 Simulator

* Download the latest release of the
[IBM TPM 1.2 Simulator](https://sourceforge.net/projects/ibmswtpm/), unpack the tarball, and `cd`
into it.
* Add `-DTPM_UNIX_DOMAIN_SOCKET` to `tpm/makefile-en-ac`.
* Build the simulator with `make -f tpm/makefile-en-ac`
* Set `TEMP_TPM=/tmp/tpm` or some other suitable temporary location for the TPM state files and Unix
  domain socket.
* Start the simulator with `TPM_PATH=${TEMP_TPM} TPM_PORT=${TEMP_TPM}/tpm.sock`

## Running the TPM 1.2 tests against the IBM TPM 1.2 Simulator

* Comment out the line `t.Skip()` in `TestTakeOwnership`. This test normally does not work on
  physical TPMs, so it is normally disabled.
* Use `TestTakeOwnership` to take ownership of the simulated TPM with `TPM_PATH=${TEMP_TPM}/tpm.sock
  go test -v ./tpm/... -run TestTakeOwnership -count=1`
* Run the full test suite with `TPM_PATH=${TEMP_TPM}/tpm.sock go test -v ./tpm/...`

## Future Improvements

* Add setup logic to the TPM 1.2 tests to take ownership of an unowned TPM under test.
* Wrap a TPM 1.2 simulator somewhere (possibly in https://github.com/google/go-tpm-tools) and
  integrate it into test setup for the TPM 1.2 tests.
* Resolve issues that necessitated the use of `t.Skip()` in current tests.
  * Either add an informative comment along with a skip when a test fails for an expected reason, or
    remove the test.
* Resolve issues with current tests that fail on the simulator (such as `TestGetAlgs`).
* Automate the use of a simulator in a Continuous Integration environment that is accessible to
  GitHub.