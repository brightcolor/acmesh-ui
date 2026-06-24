# acmesh-ui Makefile
BINARY      := acmesh-ui
PKG         := github.com/bright-color/acmesh-ui
CMD         := ./cmd/acmesh-ui
DIST        := dist

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: all build test lint vet clean release checksums run fmt

all: build

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

run: build
	./$(BINARY) serve --config ./config.yaml

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Requires golangci-lint if installed; falls back to vet.
lint:
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run || go vet ./...

clean:
	rm -rf $(BINARY) $(DIST)

# Cross-compile release artifacts for linux amd64 and arm64.
release: clean
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64 $(CMD)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64 $(CMD)
	@echo "built $(VERSION) into $(DIST)/"

checksums: release
	cd $(DIST) && sha256sum $(BINARY)-* > SHA256SUMS
	@cat $(DIST)/SHA256SUMS
