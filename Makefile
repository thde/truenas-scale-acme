APP_NAME ?= truenas-scale-acme

# try getconf (linux / macos), getconf (BSD), nproc, then fallback to 1
NPROCS := $(shell getconf _NPROCESSORS_ONLN 2>/dev/null || getconf NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || echo 1)
MAKEFLAGS += --jobs=$(NPROCS)

.PHONY: all build test clean lint update help

all: build

build:
	GITHUB_REPOSITORY=local goreleaser build --clean --snapshot --single-target

generate:
	go generate ./...

test:
	go test -race ./...

lint: mod-tidy vet staticcheck golangci-lint modernize govulncheck gorelease-check

lint-fix:
	go mod tidy
	golangci-lint run --fix
	go fix ./...
	typos -w
	$(MAKE) lint

mod-tidy:
	go mod tidy -diff

vet:
	go vet ./...

golangci-lint:
	golangci-lint run

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

modernize:
	go fix -diff ./...

govulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

typos:
	typos

gorelease-check:
	goreleaser check

update:
	go get -v -u ./...
	go mod tidy
	$(MAKE) test

clean:
	rm -rf dist/

help:
	@echo "make           # Build $(APP_NAME)"
	@echo "make test      # Run tests"
	@echo "make generate  # Generate code"
	@echo "make lint-fix  # Run linters and try fix issues"
	@echo "make lint      # Run linters"
	@echo "make update    # Update dependencies"
	@echo "make clean     # Remove built app"
