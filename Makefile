# Tool versions
GOLANGCI_VERSION ?= v1.64.8
SCRATCH_VERSION ?= 0.8.1
MOCKERY_VERSION ?= v3.7.0

# Coverage directory
COVERAGE_DIR := coverage

# test-runner binary (auto-built from source if needed)
TEST_RUNNER     := dev/tools/bin/test-runner
TEST_RUNNER_SRC := dev/tools/test-runner-main.go

# Include dev tools
-include dev/tools/tools.mk

$(TEST_RUNNER): $(TEST_RUNNER_SRC)
	@mkdir -p dev/tools/bin
	@echo "Building test-runner..."
	@go build -o $(TEST_RUNNER) $(TEST_RUNNER_SRC)

.PHONY: test test-race bench coverage lint lint-fix fmt vet audit tidy clean build-examples help init-mem-bank update-mem-bank release release-dry-run release-with-notes generate-mocks

##@ Testing
test: $(TEST_RUNNER) ## Run unit tests
	@$(TEST_RUNNER) --title="Unit Tests" ./lib/...

test-race: $(TEST_RUNNER) ## Run unit tests with race detector
	@CGO_ENABLED=1 $(TEST_RUNNER) --race --title="Unit Tests (Race)" ./lib/...

bench: ## Run benchmarks
	@echo "📊 Running benchmarks..."
	go test -bench=. -benchmem ./lib/...

coverage: $(TEST_RUNNER) ## Generate coverage report
	@mkdir -p $(COVERAGE_DIR)
	@$(TEST_RUNNER) --coverprofile=$(COVERAGE_DIR)/coverage.out --title="Coverage" ./lib/...
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "📊 Coverage: $$(go tool cover -func $(COVERAGE_DIR)/coverage.out | grep total | awk '{print $$3}')"

##@ Code Quality
fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: $(golangci-lint) ## Run golangci-lint
	@if [ -x "$(golangci-lint)" ]; then \
		$(golangci-lint) run ./... --timeout=5m; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... --timeout=5m; \
	else \
		echo "golangci-lint not found. Run 'make tools-setup' to install tools."; \
		exit 1; \
	fi

lint-fix: ## Run golangci-lint with auto-fix
	@if [ -x "$(golangci-lint)" ]; then \
		$(golangci-lint) run ./... --timeout=5m --fix; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... --timeout=5m --fix; \
	else \
		echo "golangci-lint not found. Run 'make tools-setup' to install tools."; \
		exit 1; \
	fi

##@ Audit
audit: vet lint test ## Run all quality checks
	go mod verify
	go mod tidy -diff

tidy: ## Tidy dependencies
	go mod tidy

##@ Build
clean: ## Clean build artifacts
	rm -rf $(COVERAGE_DIR) bin/

build-examples: ## Build example programs
	@echo "Building examples..."
	@mkdir -p bin
	@for dir in lib/examples/*/; do \
		name=$$(basename $$dir); \
		echo "  Building example-$$name..."; \
		go build -o bin/example-$$name ./$$dir; \
	done
	@echo "✅ Examples built in bin/"

##@ Code Generation
generate-mocks: $(mockery) ## Generate mocks using mockery
	$(mockery) --config mockery.yaml

##@ Memory Bank
init-mem-bank: $(scratch) ## Initialize memory bank in project
	@echo "Initializing memory bank..."
	$(scratch) init-mem-bank

update-mem-bank: $(scratch) ## Update memory bank templates
	@echo "Updating memory bank..."
	$(scratch) update-mem-bank

##@ Release
release: ## Create a new release (interactive)
	@chmod +x dev/tools/release.sh
	@./dev/tools/release.sh

release-dry-run: ## Preview release without making changes
	@chmod +x dev/tools/release.sh
	@./dev/tools/release.sh --dry-run

release-with-notes: ## Create release with AI-generated notes (requires claude CLI)
	@chmod +x dev/tools/release.sh
	@./dev/tools/release.sh --generate-notes

##@ Help
help: ## Show this help
	@echo ""
	@printf '\033[36m╔═══════════════════════════════════════════════════════════════╗\033[0m\n'
	@printf '\033[36m║  %-61s║\033[0m\n' "auth-client-go - Makefile commands"
	@printf '\033[36m╚═══════════════════════════════════════════════════════════════╝\033[0m\n'
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[0m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[33m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
	@echo ""

.DEFAULT_GOAL := help
