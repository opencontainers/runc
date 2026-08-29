# runc Integration Tests

Integration tests provide end-to-end testing of runc.

Note that integration tests do **not** replace unit tests.

As a rule of thumb, code should be tested thoroughly with unit tests.
Integration tests on the other hand are meant to test a specific feature end
to end.

Integration tests are written in *bash* using the
[bats (Bash Automated Testing System)](https://github.com/bats-core/bats-core)
framework. Please see
[bats documentation](https://bats-core.readthedocs.io/en/stable/index.html)
for more details.

## Running integration tests

The easiest way to run integration tests is with Docker:
```bash
make integration
```
Alternatively, you can run integration tests directly on your host through make:
```bash
sudo make localintegration
```
Or you can just run them directly using bats
```bash
sudo bats tests/integration
```
To run a single test bucket:
```bash
make integration TESTPATH="/checkpoint.bats"
```


To run them on your host, you need to set up a development environment plus
[bats (Bash Automated Testing System)](https://github.com/bats-core/bats-core#installing-bats-from-source).

For example:
```bash
cd ~/go/src/github.com
git clone https://github.com/bats-core/bats-core.git
cd bats-core
./install.sh /usr/local
```

## Writing integration tests

[Helper functions](https://github.com/opencontainers/runc/blob/master/tests/integration/helpers.bash)
are provided in order to facilitate writing tests.

Call runc (and any other command) via bats' `run` helper, giving the expected
exit code, and check its output with the assertion helpers:

```bash
run -0 runc list -q
assert_line --index 0 "test_box1"

run ! runc state test_busybox
assert_output --partial "does not exist"
```

The `-N` argument to `run` makes bats fail the test unless the command exits
with code N, and `run !` unless it fails with any non-zero code. Use `run -0`
for a command that has to succeed; a bare `run` accepts any exit code, so use
it only where the exit code is genuinely irrelevant.

The available assertions, a subset of the
[bats-assert](https://github.com/bats-core/bats-assert) API implemented in
[lib/assert.bash](https://github.com/opencontainers/runc/blob/master/tests/integration/lib/assert.bash),
are:

```bash
assert_output [--partial | --regexp] EXPECTED
refute_output [--partial | --regexp] UNEXPECTED
assert_line --index IDX [--partial | --regexp] EXPECTED
```

Prefer these over `[ ... ]` or `[[ ... ]]` comparisons: when an assertion
fails, the test log shows both the expected and the actual value, while a bare
test expression only shows the expression itself.

Please see existing tests for examples.
