#!/usr/bin/env bash
set -e

# This script runs all validations

validate() {
    sed -i 's!docker/docker!docker/libcontainer!' /go/src/github.com/docker/docker/hack/make/.validate
    /go/src/github.com/docker/docker/hack/make/validate-dco
    /go/src/github.com/docker/docker/hack/make/validate-gofmt
}

# run validations
validate
