BINARY_NAME=bin/hawk.catcher
BINARY_NAME_LINUX=$(BINARY_NAME)
BINARY_NAME_WINDOWS=$(BINARY_NAME).exe
BINARY_NAME_DARWIN=$(BINARY_NAME)

DOCKER_IMAGE=hawk.catcher

all: check lint build

build:
	go build -o $(BINARY_NAME) -v .
check:
	gometalinter --vendor --fast --enable-gc --tests --aggregate --disable=gotype --disable=gosec .
lint:
	golint ./cmd/... ./lib/... .
clean:
	go clean
	rm -rf $(BINARY_NAME)
	rm -rf $(BINARY_NAME_LINUX)
	rm -rf $(BINARY_NAME_WINDOWS)
	rm -rf $(BINARY_NAME_DARWIN)

build-all: build-linux build-windows build-darwin

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME_LINUX) -v .

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME_WINDOWS) -v .

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME_DARWIN) -v .


docker: docker-build docker-run

docker-build:
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .
docker-run:
	docker-compose up