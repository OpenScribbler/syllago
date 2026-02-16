.PHONY: build test lint clean

build:
	go build ./cmd/...

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
