BINARY_NAME=bin/hawk.collector
BINARY_NAME_LINUX=$(BINARY_NAME)-linux
BINARY_NAME_WINDOWS=$(BINARY_NAME)-windows.exe
BINARY_NAME_DARWIN=$(BINARY_NAME)-darwin
DOCKER_IMAGE=hawk.collector

export GO111MODULE=on

GOLANGCI_LINT_VERSION=v1.63.4
GOLANGCI_LINT=bin/golangci-lint

$(GOLANGCI_LINT):
	@echo "Installing golangci-lint..."
	@mkdir -p bin
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./bin $(GOLANGCI_LINT_VERSION)

all: lint build

build:
	go build -o $(BINARY_NAME) -v ./
	chmod +x $(BINARY_NAME)
test:
	go test ./...
lint: $(GOLANGCI_LINT)
	./$(GOLANGCI_LINT) run ./...
clean:
	go clean
	rm -rf $(BINARY_NAME)
	rm -rf $(BINARY_NAME_LINUX)
	rm -rf $(BINARY_NAME_WINDOWS)
	rm -rf $(BINARY_NAME_DARWIN)
run: build
	cp .env ./bin/.env
	./bin/hawk.collector run

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