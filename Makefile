
all:
	docker build -t docker/libcontainer .

test: build
       docker run --rm --privileged docker/libcontainer

sh:
	docker run --rm -it --cap-add NET_ADMIN --cap-add SYS_ADMIN -w /busybox docker/libcontainer nsinit exec sh

GO_PACKAGES = $(shell find . -not \( -wholename ./vendor -prune \) -name '*.go' -print0 | xargs -0n1 dirname | sort -u)

direct-test:
	go test -cover -v $(GO_PACKAGES)

direct-test-short:
	go test -cover -test.short -v $(GO_PACKAGES)

direct-build:
	go build -v $(GO_PACKAGES)

direct-install:
	go install -v $(GO_PACKAGES)
