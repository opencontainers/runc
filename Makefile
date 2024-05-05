CONTAINER_ENGINE := docker
GO ?= go

PREFIX ?= /usr/local
BINDIR := $(PREFIX)/sbin
MANDIR := $(PREFIX)/share/man

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_BRANCH_CLEAN := $(shell echo $(GIT_BRANCH) | sed -e "s/[^[:alnum:]]/-/g")
RUNC_IMAGE := runc_dev$(if $(GIT_BRANCH_CLEAN),:$(GIT_BRANCH_CLEAN))
PROJECT := github.com/opencontainers/runc
BUILDTAGS ?= seccomp

COMMIT ?= $(shell git describe --dirty --long --always)
VERSION ?= $(shell cat ./VERSION)
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
		LDFLAGS_STATIC := -linkmode external -extldflags --static-pie
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

runc:
	$(GO_BUILD) -o runc .

all: runc recvtty sd-helper seccompagent

recvtty sd-helper seccompagent:
	$(GO_BUILD) -o contrib/cmd/$@/$@ ./contrib/cmd/$@

static:
	$(GO_BUILD_STATIC) -o runc .

releaseall: RELEASE_ARGS := "-a arm64 -a armel -a armhf -a ppc64le -a riscv64 -a s390x"
releaseall: release

release: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--rm -v $(CURDIR):/go/src/$(PROJECT) \
		-e RELEASE_ARGS=$(RELEASE_ARGS) \
		$(RUNC_IMAGE) make localrelease
	script/release_sign.sh -S $(GPG_KEYID) -r release/$(VERSION) -v $(VERSION)

localrelease: verify-changelog
	script/release_build.sh -r release/$(VERSION) -v $(VERSION) $(RELEASE_ARGS)

dbuild: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make clean all

lint:
	golangci-lint run ./...

man:
	man/md2man-all.sh

runcimage:
	$(CONTAINER_ENGINE) build $(CONTAINER_ENGINE_BUILD_FLAGS) -t $(RUNC_IMAGE) .

test: unittest integration rootlessintegration

localtest: localunittest localintegration localrootlessintegration

unittest: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v /lib/modules:/lib/modules:ro \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make localunittest TESTFLAGS=$(TESTFLAGS)

localunittest: all
	$(GO) test -timeout 3m -tags "$(BUILDTAGS)" $(TESTFLAGS) -v ./...

integration: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v /lib/modules:/lib/modules:ro \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) make localintegration TESTPATH=$(TESTPATH)

localintegration: all
	bats -t tests/integration$(TESTPATH)

rootlessintegration: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-t --privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		-e ROOTLESS_TESTPATH \
		$(RUNC_IMAGE) make localrootlessintegration

localrootlessintegration: all
	tests/rootless.sh

shell: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		-ti --privileged --rm \
		-v $(CURDIR):/go/src/$(PROJECT) \
		$(RUNC_IMAGE) bash

install:
	install -D -m0755 runc $(DESTDIR)$(BINDIR)/runc

install-bash:
	install -D -m0644 contrib/completions/bash/runc $(DESTDIR)$(PREFIX)/share/bash-completion/completions/runc

install-man: man
	install -d -m 755 $(DESTDIR)$(MANDIR)/man8
	install -D -m 644 man/man8/*.8 $(DESTDIR)$(MANDIR)/man8

clean:
	rm -f runc runc-*
	rm -f contrib/cmd/recvtty/recvtty
	rm -f contrib/cmd/sd-helper/sd-helper
	rm -f contrib/cmd/seccompagent/seccompagent
	rm -rf release
	rm -rf man/man8

cfmt: C_SRC=$(shell git ls-files '*.c' | grep -v '^vendor/')
cfmt:
	indent -linux -l120 -il0 -ppi2 -cp1 -T size_t -T jmp_buf $(C_SRC)

shellcheck:
	shellcheck tests/integration/*.bats tests/integration/*.sh \
		tests/integration/*.bash tests/*.sh \
		man/*.sh script/*
	# TODO: add shellcheck for more sh files (contrib/completions/bash/runc).

shfmt:
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--rm -v $(CURDIR):/src -w /src \
		mvdan/shfmt:v3.5.1 -d -w .

localshfmt:
	shfmt -d -w .

vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

verify-changelog:
	# No space at EOL.
	! grep -n '\s$$' CHANGELOG.md
	# Period before issue/PR references.
	! grep -n '[0-9a-zA-Z][^.] (#[1-9][0-9, #]*)$$' CHANGELOG.md

verify-dependencies: vendor
	@test -z "$$(git status --porcelain -- go.mod go.sum vendor/)" \
		|| (echo -e "git status:\n $$(git status -- go.mod go.sum vendor/)\nerror: vendor/, go.mod and/or go.sum not up to date. Run \"make vendor\" to update"; exit 1) \
		&& echo "all vendor files are up to date."

validate-keyring:
	script/keyring_validate.sh

.PHONY: runc all recvtty sd-helper seccompagent static releaseall release \
	localrelease dbuild lint man runcimage \
	test localtest unittest localunittest integration localintegration \
	rootlessintegration localrootlessintegration shell install install-bash \
	install-man clean cfmt shfmt localshfmt shellcheck \
	vendor verify-changelog verify-dependencies validate-keyring
