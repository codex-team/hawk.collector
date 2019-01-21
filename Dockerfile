FROM golang:stretch as builder
ARG BUILD_DIRECTORY=/build

# enable go modules
ENV GO111MODULE=on
ENV CGO_ENABLED=0

# now copy your app to the build path
RUN mkdir $BUILD_DIRECTORY
ADD ./catcher $BUILD_DIRECTORY

# should be able to build now
WORKDIR $BUILD_DIRECTORY
RUN go build -o hawk.catcher .

FROM alpine
ARG BUILD_DIRECTORY=/build

WORKDIR /app
COPY --from=builder $BUILD_DIRECTORY .
COPY ./tests/docker-config.json .

EXPOSE 3000
CMD ["./hawk.catcher", "run", "-C", "docker-config.json"]
