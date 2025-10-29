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
	runc -0 run test_hello
}

@test "spec generation --bundle" {
	runc -0 run --bundle "$(pwd)" test_hello
}

@test "spec validator" {
	requires rootless_no_features

	SPEC_VERSION=$(awk '$1 == "github.com/opencontainers/runtime-spec" {print $2}' "$BATS_TEST_DIRNAME"/../../go.mod)
	# Will look like this when not pinned to specific tag: "v0.0.0-20190207185410-29686dbc5559", otherwise "v1.0.0"
	SPEC_COMMIT=$(cut -d "-" -f 3 <<<"$SPEC_VERSION")
	SPEC_REF=$([[ -z "$SPEC_COMMIT" ]] && echo "$SPEC_VERSION" || echo "$SPEC_COMMIT")

	git clone https://github.com/opencontainers/runtime-spec.git
	(cd runtime-spec && git reset --hard "$SPEC_REF")

	cd runtime-spec/schema
	go mod init runtime-spec
	go mod tidy
	go build ./validate.go

	./validate config-schema.json ../../config.json
}
