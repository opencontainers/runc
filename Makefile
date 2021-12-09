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
VERSION := $(shell cat ./VERSION)

ifeq ($(shell $(GO) env GOOS),linux)
	ifeq (,$(filter $(shell $(GO) env GOARCH),mips mipsle mips64 mips64le ppc64))
		ifeq (,$(findstring -race,$(EXTRA_FLAGS)))
			GO_BUILDMODE := "-buildmode=pie"
		endif
	endif
endif
GO_BUILD := $(GO) build -trimpath $(GO_BUILDMODE) $(EXTRA_FLAGS) -tags "$(BUILDTAGS)" \
	-ldflags "-X main.gitCommit=$(COMMIT) -X main.version=$(VERSION) $(EXTRA_LDFLAGS)"
GO_BUILD_STATIC := CGO_ENABLED=1 $(GO) build -trimpath $(EXTRA_FLAGS) -tags "$(BUILDTAGS) netgo osusergo" \
	-ldflags "-extldflags -static -X main.gitCommit=$(COMMIT) -X main.version=$(VERSION) $(EXTRA_LDFLAGS)"

GPG_KEYID ?= asarai@suse.de

.DEFAULT: runc

runc:
	$(GO_BUILD) -o runc .

all: runc recvtty sd-helper seccompagent

recvtty sd-helper seccompagent:
	$(GO_BUILD) -o contrib/cmd/$@/$@ ./contrib/cmd/$@

static:
	$(GO_BUILD_STATIC) -o runc .

releaseall: RELEASE_ARGS := "-a arm64 -a armel -a armhf -a ppc64le -a s390x"
releaseall: release

release: runcimage
	$(CONTAINER_ENGINE) run $(CONTAINER_ENGINE_RUN_FLAGS) \
		--rm -v $(CURDIR):/go/src/$(PROJECT) \
		-e RELEASE_ARGS=$(RELEASE_ARGS) \
		$(RUNC_IMAGE) make localrelease
	script/release_sign.sh -S $(GPG_KEYID) -r release/$(VERSION) -v $(VERSION)

localrelease:
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
		script/release_*.sh script/seccomp.sh script/lib.sh
	# TODO: add shellcheck for more sh files

shfmt:
	shfmt -ln bats -d -w tests/integration/*.bats
	shfmt -ln bash -d -w man/*.sh script/* tests/*.sh tests/integration/*.bash

vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

verify-dependencies: vendor
	@test -z "$$(git status --porcelain -- go.mod go.sum vendor/)" \
		|| (echo -e "git status:\n $$(git status -- go.mod go.sum vendor/)\nerror: vendor/, go.mod and/or go.sum not up to date. Run \"make vendor\" to update"; exit 1) \
		&& echo "all vendor files are up to date."

.PHONY: runc all recvtty sd-helper seccompagent static releaseall release \
	localrelease dbuild lint man runcimage \
	test localtest unittest localunittest integration localintegration \
	rootlessintegration localrootlessintegration shell install install-bash \
	install-man clean cfmt shfmt shellcheck \
	vendor verify-dependencies
