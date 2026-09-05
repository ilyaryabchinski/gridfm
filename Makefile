.PHONY: build lint lint-fix fmt vet test bench check clean install release distclean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Reproducible build flags: no cgo, no build paths, version stamped from
# the tag. The archives are created with normalized metadata so the same
# tree always produces byte-identical artifacts.
BUILD_FLAGS = -trimpath -buildvcs=true -ldflags "-s -w -X main.version=$(VERSION)"
TAR_FLAGS = --sort=name --owner=0 --group=0 --numeric-owner --mtime='UTC 1970-01-01'

build:
	go build $(BUILD_FLAGS) -o bin/gridfm ./cmd/gridfm

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

bench:
	go test -run xxx -bench . -benchmem ./...

check: vet lint test

clean:
	rm -rf bin dist

install: build
	install -Dm755 bin/gridfm ~/.local/bin/gridfm
	install -Dm644 docs/gridfm.1 ~/.local/share/man/man1/gridfm.1

# release builds the release matrix into dist/ with checksums. CI runs
# the same recipe on a version tag.
release:
	rm -rf dist
	@mkdir -p dist
	set -e; for arch in amd64 arm64; do \
		mkdir -p dist/gridfm-$(VERSION)-linux-$$arch; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch \
			go build $(BUILD_FLAGS) -o dist/gridfm-$(VERSION)-linux-$$arch/gridfm ./cmd/gridfm; \
		cp README.md docs/gridfm.1 dist/gridfm-$(VERSION)-linux-$$arch/; \
		tar $(TAR_FLAGS) -czf dist/gridfm-$(VERSION)-linux-$$arch.tar.gz -C dist gridfm-$(VERSION)-linux-$$arch; \
	done
	cd dist && sha256sum *.tar.gz > checksums.txt
	@echo "release artifacts in dist/"

distclean: clean
