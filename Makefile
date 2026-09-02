.PHONY: build lint lint-fix fmt vet test check clean

build:
	go build -o bin/gridfm ./cmd/gridfm

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

fmt:
	golangci-lint fmt

vet:
	go vet ./...

test:
	go test ./...

check: vet lint test

clean:
	rm -rf bin
