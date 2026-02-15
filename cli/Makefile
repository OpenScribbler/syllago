BINARY := nesco
REPO_ROOT := $(shell cd .. && pwd)
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
VERSION := $(shell cat ../VERSION 2>/dev/null || echo dev)
LDFLAGS := -X main.repoRoot=$(REPO_ROOT) -X main.buildCommit=$(COMMIT) -X main.version=$(VERSION)

.PHONY: build clean build-all test fmt vet

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/nesco

clean:
	rm -f $(BINARY) $(BINARY)-*

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 ./cmd/nesco

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64 ./cmd/nesco

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64 ./cmd/nesco

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 ./cmd/nesco

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...
