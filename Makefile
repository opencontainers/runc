RUNC_TEST_IMAGE=runc_test
PROJECT=github.com/opencontainers/runc
TEST_DOCKERFILE=script/test_Dockerfile
export GOPATH:=$(CURDIR)/Godeps/_workspace:$(GOPATH)

# allow for passing buildtags to make or setting the default
DEFAULT_BUILDTAGS=seccomp
BUILDTAGS := $(if $(BUILDTAGS),$(BUILDTAGS),$(DEFAULT_BUILDTAGS))

all:
	go build -tags "$(BUILDTAGS)" -o runc .

static:
	CGO_ENABLED=1 go build -tags "$(BUILDTAGS) cgo static_build" -ldflags "-w -extldflags -static" -o runc .

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
	go test -tags "$(BUILDTAGS)" ${TESTFLAGS} -v ./...


install:
	cp runc /usr/local/bin/runc

clean:
	rm runc

validate: vet
	script/validate-gofmt
	go vet ./...

ci: validate localtest
