FROM golang:stretch as builder
ARG BUILD_DIRECTORY=/build

# enable go modules
ENV GO111MODULE=on
ENV CGO_ENABLED=0

# now copy go.mod and go.sum to the build path
RUN mkdir $BUILD_DIRECTORY
COPY go.mod $BUILD_DIRECTORY
COPY go.sum $BUILD_DIRECTORY

# download modules (for fast build due to docker caching)
WORKDIR $BUILD_DIRECTORY
RUN go mod download

# copy app sources and build
ADD . $BUILD_DIRECTORY
RUN go build -o hawk.collector .

FROM alpine
ARG BUILD_DIRECTORY=/build

WORKDIR /app
COPY --from=builder $BUILD_DIRECTORY .
COPY .env.docker .env

EXPOSE 3000
CMD ["./hawk.collector", "run"]
