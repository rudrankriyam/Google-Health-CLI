.PHONY: build test fmt check

build:
	go build -o ghealth .

test:
	go test ./...

fmt:
	gofmt -w .

check: fmt test
	go build ./...
