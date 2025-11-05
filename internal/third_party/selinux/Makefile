GO ?= go

all: build build-cross

define go-build
	GOOS=$(1) GOARCH=$(2) $(GO) build ${BUILDFLAGS} ./...
endef

.PHONY: build
build:
	$(call go-build,linux,amd64)

.PHONY: build-cross
build-cross:
	$(call go-build,linux,386)
	$(call go-build,linux,arm)
	$(call go-build,linux,arm64)
	$(call go-build,linux,ppc64le)
	$(call go-build,linux,s390x)
	$(call go-build,linux,mips64le)
	$(call go-build,linux,riscv64)
	$(call go-build,windows,amd64)
	$(call go-build,windows,386)


.PHONY: test
test:
	$(GO) test -timeout 3m ${TESTFLAGS} -v ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: vendor
vendor:
	$(GO) mod tidy
	$(GO) mod verify
