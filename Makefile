.PHONY: bin clean test bin/runc bin/runc-linux-amd64
GOPATH := $(shell pwd)/Godeps/_workspace:$(shell pwd)/Godeps/_workspace/src/github.com/docker/libcontainer/vendor:$(GOPATH)
PATH := $(GOPATH)/bin:$(PATH)
VERSION := $(shell git describe --always --dirty --tags)
VERSION_FLAGS := -ldflags "-X main.version $(VERSION)"

install: bin/runc
	cp bin/runc /usr/local/bin/runc

bin/runc: bin
	go build -v $(VERSION_FLAGS) -o $@ ./cmd/runc

bin/runc-linux-amd64: bin
	GOOS=linux GOARCH=amd64 go build -v $(VERSION_FLAGS) -o $@ ./cmd/runc

test:
	go test -v ./...

bin:
	mkdir -p bin

clean:
	rm -rf bin /usr/local/bin/runc

