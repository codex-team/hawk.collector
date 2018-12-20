all: check lint build
build:
	go build -o bin/hawk.catcher .
check:
	gometalinter --vendor --fast --enable-gc --tests --aggregate --disable=gotype --disable=gosec .
lint:
	golint .
