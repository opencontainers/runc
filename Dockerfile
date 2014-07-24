FROM crosbymichael/golang

RUN apt-get update && apt-get install -y gcc
RUN go get code.google.com/p/go.tools/cmd/cover

# setup a playground for us to spawn containers in
RUN mkdir /busybox && \
    curl -sSL 'https://github.com/jpetazzo/docker-busybox/raw/buildroot-2014.02/rootfs.tar' | tar -xC /busybox && \
    echo "daemon:x:1:1:daemon:/sbin:/sbin/nologin" >> /busybox/etc/passwd

RUN curl -sSL https://raw.githubusercontent.com/dotcloud/docker/master/hack/dind -o /dind && \
    chmod +x /dind

COPY . /go/src/github.com/docker/libcontainer
WORKDIR /go/src/github.com/docker/libcontainer
RUN cp sample_configs/minimal.json /busybox/container.json

RUN go get -d ./... && go install ./...

ENTRYPOINT ["/dind"]
CMD ["go", "test", "-cover", "./..."]
