SHELL = /bin/bash
GO ?= go
CC ?= gcc

all: build

lint:
	golangci-lint run ./...

build:
	$(GO) build -v ./...
	# Build crit binary
	$(MAKE) -C crit bin/crit

test: build
	$(MAKE) -C test

coverage:
	$(MAKE) -C test coverage

codecov:
	$(MAKE) -C test codecov

vendor:
	GO111MODULE=on $(GO) mod tidy
	GO111MODULE=on $(GO) mod vendor
	GO111MODULE=on $(GO) mod verify

.PHONY: build test lint vendor coverage codecov
