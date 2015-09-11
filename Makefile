RUNC_TEST_IMAGE=runc_test
PROJECT=github.com/opencontainers/runc
TEST_DOCKERFILE=script/test_Dockerfile
VERSION := $(shell cat VERSION)
export GOPATH:=$(CURDIR)/Godeps/_workspace:$(GOPATH)

GITCOMMIT := $(shell git rev-parse --short HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
	GITCOMMIT := $(GITCOMMIT)-dirty
endif
CTIMEVAR=-X $(PROJECT)/version.GitCommit '$(GITCOMMIT)' -X $(PROJECT)/version.Version '$(VERSION)'
GO_LDFLAGS=-ldflags "-w $(CTIMEVAR)"

all:
	go build -o runc ${GO_LDFLAGS} .

vet:
	go get golang.org/x/tools/cmd/vet

lint: vet
	go vet ./...
	go fmt ./...

runctestimage:
	docker build -t $(RUNC_TEST_IMAGE) -f $(TEST_DOCKERFILE) .

test: runctestimage
	docker run -e TESTFLAGS --privileged --rm -v $(CURDIR):/go/src/$(PROJECT) $(RUNC_TEST_IMAGE) make localtest

localtest:
	go test ${TESTFLAGS} -v ./...

install:
	cp runc /usr/local/bin/runc

clean:
	rm -f runc

validate: vet
	script/validate-gofmt
	go vet ./...

ci: validate localtest
