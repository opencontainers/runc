RUNC_TEST_IMAGE=runc_test
PROJECT=github.com/opencontainers/runc
TEST_DOCKERFILE=test_Dockerfile
export GOPATH:=$(CURDIR)/Godeps/_workspace:$(GOPATH)

all:
	go build -o runc .

lint:
	go get golang.org/x/tools/cmd/vet
	go vet ./...
	go fmt ./...

runctestimage:
	docker build -t $(RUNC_TEST_IMAGE) -f $(TEST_DOCKERFILE) .

test: runctestimage
	docker run --privileged --rm -v $(CURDIR):/go/src/$(PROJECT) $(RUNC_TEST_IMAGE) make localtest

localtest:
	go test -v ./...

install:
	cp runc /usr/local/bin/runc

clean:
	rm runc
