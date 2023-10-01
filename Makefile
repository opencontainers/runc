SHELL = /bin/bash

CONTAINER_ENGINE := docker
GO ?= go

# Get CC values for cross-compilation.
include cc_platform.mk

PREFIX ?= /usr/local
BINDIR := $(PREFIX)/sbin
MANDIR := $(PREFIX)/share/man

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
RUNC_IMAGE := runc_dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
PROJECT := github.com/opencontainers/runc
BUILDTAGS ?= seccomp urfave_cli_no_docs
BUILDTAGS += $(EXTRA_BUILDTAGS)

COMMIT ?= $(shell git describe --dirty --long --always)
VERSION := $(shell cat ./VERSION)
LDFLAGS_COMMON := -X main.gitCommit=$(COMMIT) -X main.version=$(VERSION)

GOARCH := $(shell $(GO) env GOARCH)

GO_BUILDMODE :=
# Enable dynamic PIE executables on supported platforms.
ifneq (,$(filter $(GOARCH),386 amd64 arm arm64 ppc64le riscv64 s390x))
	ifeq (,$(findstring -race,$(EXTRA_FLAGS)))
		GO_BUILDMODE := "-buildmode=pie"
	endif
endif
GO_BUILD := $(GO) build -trimpath $(GO_BUILDMODE) \
	$(EXTRA_FLAGS) -tags "$(BUILDTAGS)" \
	-ldflags "$(LDFLAGS_COMMON) $(EXTRA_LDFLAGS)"

GO_BUILDMODE_STATIC :=
LDFLAGS_STATIC := -extldflags -static
# Enable static PIE executables on supported platforms.
# This (among the other things) requires libc support (rcrt1.o), which seems
# to be available only for arm64 and amd64 (Debian Bullseye).
ifneq (,$(filter $(GOARCH),arm64 amd64))
	ifeq (,$(findstring -race,$(EXTRA_FLAGS)))
		GO_BUILDMODE_STATIC := -buildmode=pie
		LDFLAGS_STATIC := -linkmode external -extldflags -static-pie
	endif
endif
# Enable static PIE binaries on supported platforms.
GO_BUILD_STATIC := $(GO) build -trimpath $(GO_BUILDMODE_STATIC) \
	$(EXTRA_FLAGS) -tags "$(BUILDTAGS) netgo osusergo" \
	-ldflags "$(LDFLAGS_COMMON) $(LDFLAGS_STATIC) $(EXTRA_LDFLAGS)"

GPG_KEYID ?= asarai@suse.de

# Some targets need cgo, which is disabled by default when cross compiling.
# Enable cgo explicitly for those.
# Both runc and libcontainer/integration need libcontainer/nsenter.
runc static localunittest: export CGO_ENABLED=1
# seccompagent needs libseccomp (when seccomp build tag is set).
ifneq (,$(filter $(BUILDTAGS),seccomp))
seccompagent: export CGO_ENABLED=1
endif

.DEFAULT: runc

.PHONY: runc
runc: runc-bin verify-dmz-arch

.PHONY: runc-bin
runc-bin: runc-dmz
	$(GO_BUILD) -o runc .

.PHONY: all
all: runc recvtty sd-helper seccompagent fs-idmap memfd-bind pidfd-kill

.PHONY: recvtty sd-helper seccompagent fs-idmap memfd-bind pidfd-kill
recvtty sd-helper seccompagent fs-idmap memfd-bind pidfd-kill:
	$(GO_BUILD) -o contrib/cmd/$@/$@ ./contrib/cmd/$@

.PHONY: static
static: static-bin verify-dmz-arch

.PHONY: static-bin
static-bin: runc-dmz
	$(GO_BUILD_STATIC) -o runc .

.PHONY: runc-dmz
runc-dmz:
	rm -f libcontainer/dmz/runc-dmz
	$(GO) generate -tags "$(BUILDTAGS)" ./libcontainer/dmz

.PHONY: releaseall
releaseall: RELEASE_ARGS := "-a 386 -a amd64 -a arm64 -a armel -a armhf -a ppc64le -a riscv64 -a s390x"
releaseall: release

.PHONY: release
release: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--rm -v $(CURDIR):/go/src/$(PROJECT) \
		-e RELEASE_ARGS=$(RELEASE_ARGS) \
		$(RUNC_IMAGE) make localrelease
	script/release_sign.sh -S $(GPG_KEYID) -r release/$(VERSION) -v $(VERSION)

.PHONY: localrelease
localrelease: verify-changelog
	script/release_build.sh -r release/$(VERSION) -v $(VERSION) $(RELEASE_ARGS)

.PHONY: dbuild
dbuild: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make clean all

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: man
man:
	man/md2man-all.sh

.PHONY: runcimage
runcimage:
	$(CONTAINER_ENGINE) build $(CONTAINER_ENGINE_BUILD_FLAGS) -t $(RUNC_IMAGE) .

.PHONY: test
test: unittest integration rootlessintegration

.PHONY: localtest
localtest: localunittest localintegration localrootlessintegration

.PHONY: unittest
unittest: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v /lib/modules:/lib/modules:ro \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make localunittest TESTFLAGS="$(TESTFLAGS)"

.PHONY: localunittest
localunittest: all
	$(GO) test -timeout 3m -tags "$(BUILDTAGS)" $(TESTFLAGS) -v ./...

.PHONY: integration
integration: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v /lib/modules:/lib/modules:ro \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make localintegration TESTPATH="$(TESTPATH)"

.PHONY: localintegration
localintegration: all
	bats -t tests/integration$(TESTPATH)

.PHONY: rootlessintegration
rootlessintegration: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		-e ROOTLESS_TESTPATH \
		$(RUNC_IMAGE) make localrootlessintegration

.PHONY: localrootlessintegration
localrootlessintegration: all
	tests/rootless.sh

.PHONY: shell
shell: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-ti --privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) bash

.PHONY: install
install:
	install -D -m0755 runc $(DESTDIR)$(BINDIR)/runc

.PHONY: install-bash
install-bash:
	install -D -m0644 contrib/completions/bash/runc $(DESTDIR)$(PREFIX)/share/bash-completion/completions/runc

.PHONY: install-man
install-man: man
	install -d -m 755 $(DESTDIR)$(MANDIR)/man8
	install -D -m 644 man/man8/*.8 $(DESTDIR)$(MANDIR)/man8

.PHONY: clean
clean:
	rm -f runc runc-* libcontainer/dmz/runc-dmz
	rm -f contrib/cmd/fs-idmap/fs-idmap
	rm -f contrib/cmd/recvtty/recvtty
	rm -f contrib/cmd/sd-helper/sd-helper
	rm -f contrib/cmd/seccompagent/seccompagent
	rm -f contrib/cmd/memfd-bind/memfd-bind
	rm -f contrib/cmd/pidfd-kill/pidfd-kill
	sudo rm -rf release
	rm -rf man/man8

.PHONY: cfmt
cfmt: C_SRC=$(shell git ls-files '*.c' | grep -v '^vendor/')
cfmt:
	indent -linux -l120 -il0 -ppi2 -cp1 -T size_t -T jmp_buf $(C_SRC)

.PHONY: shellcheck
shellcheck:
	shellcheck tests/integration/*.bats tests/integration/*.sh \
		tests/integration/*.bash tests/*.sh \
		man/*.sh script/*
	# TODO: add shellcheck for more sh files (contrib/completions/bash/runc).

.PHONY: shfmt
shfmt:
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--rm -v $(CURDIR):/src -w /src \
		mvdan/shfmt:v3.5.1 -d -w .

.PHONY: localshfmt
localshfmt:
	shfmt -d -w .

.PHONY: venodr
vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: verify-changelog
verify-changelog:
	# No space at EOL.
	! grep -n '\s$$' CHANGELOG.md
	# Period before issue/PR references.
	! grep -n '[0-9a-zA-Z][^.] (#[1-9][0-9, #]*)$$' CHANGELOG.md

.PHONY: verify-dependencies
verify-dependencies: vendor
	@test -z "$$(git status --porcelain -- go.mod go.sum vendor/)" \
		|| (echo -e "git status:\n $$(git status -- go.mod go.sum vendor/)\nerror: vendor/, go.mod and/or go.sum not up to date. Run \"make vendor\" to update"; exit 1) \
		&& echo "all vendor files are up to date."

.PHONY: verify-dmz-arch
verify-dmz-arch:
	@if test -s libcontainer/dmz/runc-dmz; then \
		set -Eeuo pipefail; \
		export LC_ALL=C; \
		diff -u \
			<(readelf -h runc | grep -E "(Machine|Flags):") \
			<(readelf -h libcontainer/dmz/runc-dmz | grep -E "(Machine|Flags):"); \
	fi

.PHONY: validate-keyring
validate-keyring:
	script/keyring_validate.sh
