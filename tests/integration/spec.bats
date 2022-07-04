#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	update_config '.process.args = ["/bin/echo", "Hello World"]'
}

function teardown() {
	teardown_bundle
}

@test "spec generation cwd" {
	runc run test_hello
	[ "$status" -eq 0 ]
}

@test "spec generation --bundle" {
	runc run --bundle "$(pwd)" test_hello
	[ "$status" -eq 0 ]
}

@test "spec validator" {
	requires rootless_no_features

	SPEC_VERSION=$(awk '$1 == "github.com/opencontainers/runtime-spec" {print $2}' "$BATS_TEST_DIRNAME"/../../go.mod)
	# Will look like this when not pinned to specific tag: "v0.0.0-20190207185410-29686dbc5559", otherwise "v1.0.0"
	SPEC_COMMIT=$(cut -d "-" -f 3 <<<"$SPEC_VERSION")
	SPEC_REF=$([[ -z "$SPEC_COMMIT" ]] && echo "$SPEC_VERSION" || echo "$SPEC_COMMIT")

	git clone https://github.com/opencontainers/runtime-spec.git
	(cd runtime-spec && git reset --hard "$SPEC_REF")
	SCHEMA='runtime-spec/schema/config-schema.json'
	[ -e "$SCHEMA" ]

	GO111MODULE=auto go get github.com/xeipuuv/gojsonschema
	GO111MODULE=auto go build runtime-spec/schema/validate.go

	./validate "$SCHEMA" config.json
}
