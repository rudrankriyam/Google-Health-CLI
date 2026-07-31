.PHONY: build test fmt check

build:
	go build -o ghealth ./cmd/ghealth

test:
	go test ./...

fmt:
	gofmt -w .

check: fmt test
	go build ./...
