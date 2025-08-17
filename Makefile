.PHONY: lint test fmt fmt-check vet build

BIN_DIR := $(shell go env GOBIN)
ifeq ($(BIN_DIR),)
BIN_DIR := $(shell go env GOPATH)/bin
endif

BUILD_DIR = build

# Build target
BINARY_NAME = swolegen-api
BUILT_BINARY = $(BUILD_DIR)/$(BINARY_NAME)

GOCMD ?= go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test

GOLANGCI := $(BIN_DIR)/golangci-lint
GOLANGCI_VERSION := v1.65.2

# Format all packages
fmt:
	@$(GO) fmt ./...

# Check formatting without modifying files; fails if any files need formatting
fmt-check:
	@echo "Checking formatting..."
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "The following files are not gofmt-formatted:"; \
		echo "$$files"; \
		exit 1; \
	else \
		echo "All Go files are properly formatted."; \
	fi

# Run go vet for static analysis
vet:
	@$(GO) vet ./...

lint: fmt-check vet install-golangci
	$(GOLANGCI) run --config .golangci.yml

build:
	$(GOBUILD) -o $(BUILT_BINARY) -v ./cmd/swolegen-api

test:
	$(GOTEST) -v ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./...
