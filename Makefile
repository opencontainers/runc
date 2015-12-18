RUNC_IMAGE=runc_dev
RUNC_TEST_IMAGE=runc_test
PROJECT=github.com/opencontainers/runc
TEST_DOCKERFILE=script/test_Dockerfile
BUILDTAGS=seccomp
RUNC_BUILD_PATH=/go/src/github.com/opencontainers/runc/runc
RUNC_INSTANCE=runc_dev
export GOPATH:=$(CURDIR)/Godeps/_workspace:$(GOPATH)

.PHONY=dbuild

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

dbuild: runctestimage 
	docker build -t $(RUNC_IMAGE) .
	docker create --name=$(RUNC_INSTANCE) $(RUNC_IMAGE)
	docker cp $(RUNC_INSTANCE):$(RUNC_BUILD_PATH) .
	docker rm $(RUNC_INSTANCE)

install:
	cp runc /usr/local/bin/runc

clean:
	rm runc

validate: vet
	script/validate-gofmt
	go vet ./...

ci: validate localtest
