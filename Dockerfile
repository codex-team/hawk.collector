FROM golang:alpine

ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

# now copy your app to the proper build path
RUN mkdir -p $GOPATH/src/app
ADD . $GOPATH/src/app

# should be able to build now
WORKDIR $GOPATH/src/app
RUN go build -o hawk.catcher .
RUN chmod +x ./tools/start.sh
CMD ["./hawk.catcher"]