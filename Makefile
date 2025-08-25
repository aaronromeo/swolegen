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

JSONSCHEMA := jsonschema
JQ ?= jq
JQ_SCRIPT   ?= scripts/refs-to-defs.jq
NORMALIZED_SCHEMA_DIR ?= internal/llm/schemas

SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c

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

.PHONY: install-jsonschema
install-jsonschema:
# 	@if [ -x "$(JSONSCHEMA)" ]; then \
# 		echo "jsonschema found at $(JSONSCHEMA)"; \
# 	else \
# 		echo "Installing jsonschema ..."; \
# 		brew install sourcemeta/apps/jsonschema; \
# 		[ -x "$(JSONSCHEMA)" ] && echo "Installed jsonschema at $(JSONSCHEMA)" || (echo "Install failed"; exit 1); \
# 	fi


.PHONY: install-go-jsonschema
install-go-jsonschema:
	@if [ -x "$(GOJSONSCHEMA)" ]; then \
		echo "go-jsonschema found at $(GOJSONSCHEMA)"; \
	else \
		echo "Installing go-jsonschema $(GOJSONSCHEMA_VERSION) ..."; \
		go install github.com/atombender/go-jsonschema@$(GOJSONSCHEMA_VERSION); \
		[ -x "$(GOJSONSCHEMA)" ] && echo "Installed go-jsonschema at $(GOJSONSCHEMA)" || (echo "Install failed"; exit 1); \
	fi

.PHONY: check-jq
check-jq:
	@command -v $(JQ) >/dev/null || { echo "Error: '$(JQ)' not found. Install jq."; exit 1; }

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

.PHONY: normalize-schemas
normalize-schemas: check-jq install-jsonschema
	rm -f $(NORMALIZED_SCHEMA_DIR)/*.json

	$(JSONSCHEMA) bundle schemas/history-v1.json -r schemas/ > $(NORMALIZED_SCHEMA_DIR)/history-v1.json
	$(JSONSCHEMA) bundle schemas/analyzer-v1.json -r schemas/ > $(NORMALIZED_SCHEMA_DIR)/analyzer-v1.json
	$(JSONSCHEMA) bundle schemas/workout-v1.2.json -r schemas/ > $(NORMALIZED_SCHEMA_DIR)/workout-v1.2.json

	@for f in $(NORMALIZED_SCHEMA_DIR)/*.json; do \
	  echo "Processing $$f"; \
	  $(JQ) -f $(JQ_SCRIPT) "$$f" > "$$f.tmp" && mv "$$f.tmp" "$$f"; \
	done

.PHONY: generate
generate: install-go-jsonschema
	@echo "Generating schemas..."
	@echo $(GOJSONSCHEMA)

	$(GOJSONSCHEMA) \
	--schema-package=https://swolegen.app/schemas/history-v1.json=github.com/aaronromeo/swolegen/internal/llm/generated \
	--schema-output=https://swolegen.app/schemas/history-v1.json=internal/llm/generated/models.go \
	--schema-package=https://swolegen.app/schemas/analyzer-v1.json=github.com/aaronromeo/swolegen/internal/llm/generated \
	--schema-output=https://swolegen.app/schemas/analyzer-v1.json=internal/llm/generated/models.go \
	$(NORMALIZED_SCHEMA_DIR)/history-v1.json \
	$(NORMALIZED_SCHEMA_DIR)/analyzer-v1.json

	$(GOJSONSCHEMA) --schema-package=https://swolegen.app/schemas/workout-v1.2.json=github.com/aaronromeo/swolegen/internal/llm/generated \
	--schema-output=https://swolegen.app/schemas/workout-v1.2.json=internal/llm/generated/workout.go \
	$(NORMALIZED_SCHEMA_DIR)/workout-v1.2.json

.PHONY: normalize-generate
normalize-generate: normalize-schemas generate

.PHONY: build
build:
	$(GOBUILD) -o $(BUILT_BINARY) -v ./cmd/swolegen-api

.PHONY: test
test:
	$(GOTEST) -v ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./...
