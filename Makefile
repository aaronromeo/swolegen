.PHONY: lint test fmt fmt-check vet

GO ?= go

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

# Lint aggregates static checks
lint: fmt-check vet

# Run unit tests for all packages
# Use: make test or VERBOSE=1 make test
ifdef VERBOSE
TEST_FLAGS=-v
endif

test:
	@$(GO) test $(TEST_FLAGS) ./...
