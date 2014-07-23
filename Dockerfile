FROM crosbymichael/golang

RUN apt-get update && apt-get install -y gcc

ADD . /go/src/github.com/docker/libcontainer
WORKDIR /go/src/github.com/docker/libcontainer
RUN go get -d ./... && go install ./...

CMD ["go", "test", "./..."]
