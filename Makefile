.PHONY: fmt build run test clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

fmt:
	go fmt ./...

build:
	go build $(LDFLAGS) -o bin/zoea ./cmd/zoea

run: build
	./bin/zoea

test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -rf bin/ coverage.out
