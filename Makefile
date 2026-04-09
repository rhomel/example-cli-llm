BINARY := bin/example

.PHONY: build test

build:
	mkdir -p bin
	go build -o $(BINARY) .

test:
	go test ./...
