FROM golang:alpine

ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

# now copy your app to the proper build path
RUN mkdir -p $GOPATH/src/github.com/codex-team/hawk.catcher
ADD . $GOPATH/src/github.com/codex-team/hawk.catcher

# should be able to build now
WORKDIR $GOPATH/src/github.com/codex-team/hawk.catcher
RUN go build -o hawk.catcher .
CMD ["./hawk.catcher", "run", "-C", "docker-config.json"]