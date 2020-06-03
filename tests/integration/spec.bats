#!/usr/bin/env bats

load helpers

function setup() {
  # initial cleanup in case a prior test exited and did not cleanup
  cd "$INTEGRATION_ROOT"
  run rm -f -r "$HELLO_BUNDLE"

  # setup hello-world for spec generation testing
  run mkdir "$HELLO_BUNDLE"
  run mkdir "$HELLO_BUNDLE"/rootfs
  run tar -C "$HELLO_BUNDLE"/rootfs -xf "$HELLO_IMAGE"
}

function teardown() {
  cd "$INTEGRATION_ROOT"
  run rm -f -r "$HELLO_BUNDLE"
}

@test "spec generation cwd" {
  cd "$HELLO_BUNDLE"
  # note this test runs from the bundle not the integration root

  # test that config.json does not exist after the above partial setup
  [ ! -e config.json ]

  # test generation of spec does not return an error
  runc_spec
  [ "$status" -eq 0 ]

  # test generation of spec created our config.json (spec)
  [ -e config.json ]

  # test existence of required args parameter in the generated config.json
  run bash -c "grep -A2 'args' config.json | grep 'sh'"
  [[ "${output}" == *"sh"* ]]

  # change the default args parameter from sh to hello
  update_config '(.. | select(.? == "sh")) |= "/hello"'

  # ensure the generated spec works by running hello-world
  runc run test_hello
  [ "$status" -eq 0 ]
}

@test "spec generation --bundle" {
  # note this test runs from the integration root not the bundle

  # test that config.json does not exist after the above partial setup
  [ ! -e "$HELLO_BUNDLE"/config.json ]

  # test generation of spec does not return an error
  runc_spec "$HELLO_BUNDLE"
  [ "$status" -eq 0 ]

  # test generation of spec created our config.json (spec)
  [ -e "$HELLO_BUNDLE"/config.json ]

  # change the default args parameter from sh to hello
  update_config '(.. | select(.? == "sh")) |= "/hello"' $HELLO_BUNDLE

  # ensure the generated spec works by running hello-world
  runc run --bundle "$HELLO_BUNDLE" test_hello
  [ "$status" -eq 0 ]
}

@test "spec validator" {
  TESTDIR=$(pwd)
  cd "$HELLO_BUNDLE"

  run git clone https://github.com/opencontainers/runtime-spec.git src/runtime-spec
  [ "$status" -eq 0 ]

  SPEC_VERSION=$(grep 'github.com/opencontainers/runtime-spec' ${TESTDIR}/../../go.mod | cut -d ' ' -f 2)

  # Will look like this when not pinned to spesific tag: "v0.0.0-20190207185410-29686dbc5559", otherwise "v1.0.0"
  SPEC_COMMIT=$(cut -d "-" -f 3 <<< $SPEC_VERSION)

  SPEC_REF=$([[ -z "$SPEC_COMMIT" ]] && echo $SPEC_VERSION || echo $SPEC_COMMIT)

  run bash -c "cd src/runtime-spec && git reset --hard ${SPEC_REF}"

  [ "$status" -eq 0 ]
  [ -e src/runtime-spec/schema/config-schema.json ]

  run bash -c "GOPATH='$GOPATH' go get github.com/xeipuuv/gojsonschema"
  [ "$status" -eq 0 ]

  run bash -c "cd ${GOPATH}/src/github.com/xeipuuv/gojsonschema && git reset --hard 6637feb73ee44cd4640bb3def285c29774234c7f"
  [ "$status" -eq 0 ]

  GOPATH="$GOPATH" go build src/runtime-spec/schema/validate.go
  [ -e ./validate ]

  runc spec
  [ -e config.json ]

  run ./validate src/runtime-spec/schema/config-schema.json config.json
  [ "$status" -eq 0 ]
  [[ "${lines[0]}" == *"The document is valid"* ]]
}
