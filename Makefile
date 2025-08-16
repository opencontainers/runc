SHELL = /bin/bash

CONTAINER_ENGINE := docker
GO ?= go

PREFIX ?= /usr/local
BINDIR := $(PREFIX)/sbin
MANDIR := $(PREFIX)/share/man

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
RUNC_IMAGE := runc_dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
PROJECT := github.com/opencontainers/runc
EXTRA_BUILDTAGS :=
BUILDTAGS := seccomp urfave_cli_no_docs
BUILDTAGS += $(EXTRA_BUILDTAGS)

COMMIT := $(shell git describe --dirty --long --always)
EXTRA_VERSION :=
LDFLAGS_COMMON := -X main.gitCommit=$(COMMIT) \
		  $(if $(strip $(EXTRA_VERSION)),-X main.extraVersion=$(EXTRA_VERSION),)

GOARCH := $(shell $(GO) env GOARCH)

# -trimpath may be required on some platforms to create reproducible builds
# on the other hand, it does strip out build information, like -ldflags, which
# some tools use to infer the version, in the absence of go information,
# which happens when you use `go build`.
# This enables someone to override by doing `make runc TRIMPATH= ` etc.
TRIMPATH := -trimpath

GO_BUILDMODE :=
# Enable dynamic PIE executables on supported platforms.
ifneq (,$(filter $(GOARCH),386 amd64 arm arm64 ppc64le riscv64 s390x))
	ifeq (,$(findstring -race,$(EXTRA_FLAGS)))
		GO_BUILDMODE := "-buildmode=pie"
	endif
endif
GO_BUILD := $(GO) build $(TRIMPATH) $(GO_BUILDMODE) \
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
GO_BUILD_STATIC := $(GO) build $(TRIMPATH) $(GO_BUILDMODE_STATIC) \
	$(EXTRA_FLAGS) -tags "$(BUILDTAGS) netgo osusergo" \
	-ldflags "$(LDFLAGS_COMMON) $(LDFLAGS_STATIC) $(EXTRA_LDFLAGS)"

GPG_KEYID ?= asarai@suse.de

RUN_IN_CONTAINER_MAJOR := 100
RUN_IN_CONTAINER_MAJOR_SECOND := 101
RUN_IN_CONTAINER_MINOR := 1

# Some targets need cgo, which is disabled by default when cross compiling.
# Enable cgo explicitly for those.
# Both runc and libcontainer/integration need libcontainer/nsenter.
runc static localunittest: export CGO_ENABLED=1
# seccompagent needs libseccomp (when seccomp build tag is set).
ifneq (,$(filter $(BUILDTAGS),seccomp))
seccompagent: export CGO_ENABLED=1
endif

tpm-helper: export CGO_ENABLED=0

.DEFAULT: runc

.PHONY: runc
runc: runc-bin

.PHONY: runc-bin
runc-bin:
	$(GO_BUILD) -o runc .

.PHONY: all
all: runc memfd-bind

.PHONY: memfd-bind
memfd-bind:
	$(GO_BUILD) -o contrib/cmd/$@/$@ ./contrib/cmd/$@

TESTBINDIR := tests/cmd/_bin
$(TESTBINDIR):
	mkdir $(TESTBINDIR)

TESTBINS := recvtty sd-helper seccompagent fs-idmap pidfd-kill remap-rootfs key_label tpm-helper
.PHONY: test-binaries $(TESTBINS)
test-binaries: $(TESTBINS)
$(TESTBINS): $(TESTBINDIR)
	$(GO_BUILD) -o $(TESTBINDIR) ./tests/cmd/$@

.PHONY: clean
clean:
	rm -f runc runc-*
	rm -f contrib/cmd/memfd-bind/memfd-bind
	rm -fr $(TESTBINDIR)
	sudo rm -rf release
	rm -rf man/man8

.PHONY: static
static: static-bin

.PHONY: static-bin
static-bin:
	$(GO_BUILD_STATIC) -o runc .

.PHONY: releaseall
releaseall: RELEASE_ARGS := "-a 386 -a amd64 -a arm64 -a armel -a armhf -a ppc64le -a riscv64 -a s390x"
releaseall: release

.PHONY: release
release: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--rm -v $(CURDIR):/go/src/$(PROJECT) \
		-e RELEASE_ARGS=$(RELEASE_ARGS) \
		$(RUNC_IMAGE) make localrelease
	script/release_sign.sh -S $(GPG_KEYID)

.PHONY: localrelease
localrelease: verify-changelog
	script/release_build.sh $(RELEASE_ARGS)

.PHONY: dbuild
dbuild: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make clean runc test-binaries

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
		--device=/dev/cuse --device-cgroup-rule "c $(RUN_IN_CONTAINER_MAJOR):$(RUN_IN_CONTAINER_MINOR) rwm" \
		-e "RUN_IN_CONTAINER_MAJOR=$(RUN_IN_CONTAINER_MAJOR)" \
		-e "RUN_IN_CONTAINER_MINOR=$(RUN_IN_CONTAINER_MINOR)" \
		$(RUNC_IMAGE) make localunittest TESTFLAGS="$(TESTFLAGS)"

.PHONY: localunittest
localunittest: test-binaries
	$(GO) test -timeout 3m -tags "$(BUILDTAGS)" $(TESTFLAGS) -v ./...

.PHONY: integration
integration: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v /lib/modules:/lib/modules:ro \
		-v $(CURDIR):/go/src/$(PROJECT) \
		--device=/dev/cuse --device-cgroup-rule "c $(RUN_IN_CONTAINER_MAJOR):$(RUN_IN_CONTAINER_MINOR) rwm" \
		--device-cgroup-rule "c $(RUN_IN_CONTAINER_MAJOR_SECOND):$(RUN_IN_CONTAINER_MINOR) rwm" \
		$(RUNC_IMAGE) make localintegration TESTPATH="$(TESTPATH)"

.PHONY: localintegration
localintegration: runc test-binaries
	bats -t tests/integration$(TESTPATH)

.PHONY: rootlessintegration
rootlessintegration: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		-e ROOTLESS_TESTPATH \
		$(RUNC_IMAGE) make localrootlessintegration

.PHONY: localrootlessintegration
localrootlessintegration: runc test-binaries
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

.PHONY: cfmt
cfmt: C_SRC=$(shell git ls-files '*.c' | grep -v '^vendor/')
cfmt:
	indent -linux -l120 -il0 -ppi2 -cp1 -sar -T size_t -T jmp_buf $(C_SRC)

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
		mvdan/shfmt:v3.11.0 -d -w .

.PHONY: localshfmt
localshfmt:
	shfmt -d -w .

.PHONY: vendor
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

.PHONY: validate-keyring
validate-keyring:
	script/keyring_validate.sh
