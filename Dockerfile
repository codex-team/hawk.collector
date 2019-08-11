FROM golang:stretch as builder
ARG BUILD_DIRECTORY=/build

# enable go modules
ENV GO111MODULE=on
ENV CGO_ENABLED=0

# now copy your app to the build path
RUN mkdir $BUILD_DIRECTORY
ADD ./collector $BUILD_DIRECTORY

# should be able to build now
WORKDIR $BUILD_DIRECTORY
RUN go build -o hawk.collector .

FROM alpine
ARG BUILD_DIRECTORY=/build

WORKDIR /app
COPY --from=builder $BUILD_DIRECTORY .
COPY ./tests/docker-config.json .

EXPOSE 3000
CMD ["./hawk.collector", "run", "-C", "docker-config.json"]
