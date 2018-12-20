all: check lint build
build:
	go build -o bin/hawk.catcher .
check:
	gometalinter --vendor --fast --enable-gc --tests --aggregate --disable=gotype --disable=gosec .
lint:
	golint .
docker-build:
	docker build -t hawk.catcher -f Dockerfile .
docker-run:
	docker-compose up