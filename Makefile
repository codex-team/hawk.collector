BINARY_NAME=bin/hawk.catcher
BINARY_NAME_LINUX=$(BINARY_NAME)-linux
BINARY_NAME_WINDOWS=$(BINARY_NAME)-windows.exe
BINARY_NAME_DARWIN=$(BINARY_NAME)-darwin

SRC_DIRECTORY=./catcher

DOCKER_IMAGE=hawk.catcher

all: check lint build

build:
	go build -o $(BINARY_NAME) -v $(SRC_DIRECTORY)
check:
	gometalinter --vendor --fast --enable-gc --tests --aggregate --disable=gotype --disable=gosec $(SRC_DIRECTORY)
lint:
	golint $(SRC_DIRECTORY)/cmd/... $(SRC_DIRECTORY)/lib/... $(SRC_DIRECTORY)
clean:
	go clean
	rm -rf $(BINARY_NAME)
	rm -rf $(BINARY_NAME_LINUX)
	rm -rf $(BINARY_NAME_WINDOWS)
	rm -rf $(BINARY_NAME_DARWIN)

build-all: build-linux build-windows build-darwin

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME_LINUX) -v $(SRC_DIRECTORY)

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME_WINDOWS) -v $(SRC_DIRECTORY)

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME_DARWIN) -v $(SRC_DIRECTORY)


docker: docker-build docker-run

docker-build:
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .
docker-run:
	docker-compose up