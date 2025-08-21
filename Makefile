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
GOVET = $(GOCMD) vet

GOLANGCI := $(BIN_DIR)/golangci-lint
GOLANGCI_VERSION := v1.65.2

GOJSONSCHEMA := $(BIN_DIR)/go-jsonschema
GOJSONSCHEMA_VERSION := v0.20.0

# Format all packages
.PHONY: fmt
fmt:
	@$(GO) fmt ./...

.PHONY: install-golangci
install-golangci:
	@if [ -x "$(GOLANGCI)" ]; then \
		echo "golangci-lint found at $(GOLANGCI)"; \
	else \
		echo "Installing golangci-lint $(GOLANGCI_VERSION) ..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN_DIR) $(GOLANGCI_VERSION); \
		"$(GOLANGCI)" version; \
	fi

.PHONY: install-go-jsonschema
install-go-jsonschema:
	@if [ -x "$(GOJSONSCHEMA)" ]; then \
		echo "go-jsonschema found at $(GOJSONSCHEMA)"; \
	else \
		echo "Installing go-jsonschema $(GOJSONSCHEMA_VERSION) ..."; \
		go install github.com/atombender/go-jsonschema@$(GOJSONSCHEMA_VERSION); \
		"$(GOJSONSCHEMA)" version; \
	fi

# Check formatting without modifying files; fails if any files need formatting
.PHONY: fmt-check
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
.PHONY: vet
vet:
	@$(GOVET) ./...

.PHONY: lint
lint: fmt-check vet install-golangci
	$(GOLANGCI) run --config .golangci.yml

.PHONY: generate
generate: install-go-jsonschema
	$(GOJSONSCHEMA) --schema-package=https://swolegen.app/schemas/analyzer-v1.json=github.com/aaronromeo/swolegen/internal/llm/schemas \
	--schema-output=https://swolegen.app/schemas/analyzer-v1.json=internal/llm/schemas/analyzer.go \
	internal/llm/schemas/analyzer-v1.json

	$(GOJSONSCHEMA) --schema-package=https://swolegen.app/schemas/workout-v1.2.json=github.com/aaronromeo/swolegen/internal/llm/schemas \
	--schema-output=https://swolegen.app/schemas/workout-v1.2.json=internal/llm/schemas/workout.go \
	internal/llm/schemas/workout-v1.2.json

.PHONY: build
build:
	$(GOBUILD) -o $(BUILT_BINARY) -v ./cmd/swolegen-api

.PHONY: test
test:
	$(GOTEST) -v ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./...
