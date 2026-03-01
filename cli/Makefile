BINARY := syllago
ifeq ($(shell go env GOOS),windows)
  EXT := .exe
endif
OUTPUT := $(BINARY)$(EXT)
REPO_ROOT := $(shell cd .. && pwd)
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
VERSION := $(shell cat ../VERSION 2>/dev/null || echo dev)
LDFLAGS := -X main.repoRoot=$(REPO_ROOT) -X main.buildCommit=$(COMMIT) -X main.version=$(VERSION)

.PHONY: build clean build-all test fmt vet build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 build-windows-arm64

build:
	go build -ldflags "$(LDFLAGS)" -o $(OUTPUT) ./cmd/syllago

clean:
	rm -f $(BINARY) $(BINARY).exe $(BINARY)-*

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 ./cmd/syllago

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64 ./cmd/syllago

build-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64 ./cmd/syllago

build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 ./cmd/syllago

build-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-windows-amd64.exe ./cmd/syllago

build-windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-windows-arm64.exe ./cmd/syllago

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 build-windows-arm64

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...
