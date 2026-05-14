BASE_PATH := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
TOOLS_DIR := $(BASE_PATH)
TOOLS_BIN := $(TOOLS_DIR)/bin
PATH := $(TOOLS_BIN):$(PATH)
export PATH

##@ Tools - Инструменты разработки

define print-tools-header
@echo ""
@echo "╔══════════════════════════════════════════════════════════════╗"
@echo "║                      Development Tools                       ║"
@echo "╚══════════════════════════════════════════════════════════════╝"
@echo ""
endef

.PHONY: tools-clean
tools-clean: ## Очистить установленные инструменты
	$(print-tools-header)
	@rm -rf $(TOOLS_BIN)
	@echo "Tools cleaned successfully"

scratch=$(TOOLS_BIN)/scratch-$(subst .,-,$(SCRATCH_VERSION))
$(scratch):
ifeq ($(SCRATCH_VERSION),dev)
	@echo "Installing scratch from main branch..."
	GOBIN=$(TOOLS_BIN) go install github.com/webdelo/scratch@main
	mv $(TOOLS_BIN)/scratch ${scratch}
else
	@echo "Installing scratch version $(SCRATCH_VERSION)..."
	@SCRATCH_VER_NUM=$(patsubst v%,%,$(SCRATCH_VERSION)); \
	OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	ARCH=$$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/'); \
	TARBALL="scratch_$${SCRATCH_VER_NUM}_$${OS}_$${ARCH}.tar.gz"; \
	URL="https://github.com/webdelo/scratch/releases/download/$(SCRATCH_VERSION)/$${TARBALL}"; \
	if curl -sSfL -o /tmp/$$TARBALL "$$URL" 2>/dev/null && [ -s /tmp/$$TARBALL ]; then \
		tar -xzf /tmp/$$TARBALL -C $(TOOLS_BIN) scratch; \
		rm -f /tmp/$$TARBALL; \
		mv $(TOOLS_BIN)/scratch ${scratch}; \
	else \
		echo "Binary release not found, installing via go install..."; \
		rm -f /tmp/$$TARBALL; \
		GOBIN=$(TOOLS_BIN) go install github.com/webdelo/scratch@$(SCRATCH_VERSION); \
		mv $(TOOLS_BIN)/scratch ${scratch}; \
	fi
endif

golangci-lint=$(TOOLS_BIN)/golangci-lint-$(subst .,-,$(GOLANGCI_VERSION))
$(golangci-lint):
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b ${TOOLS_BIN} ${GOLANGCI_VERSION}
	mv $(TOOLS_BIN)/golangci-lint ${golangci-lint}

mockery=$(TOOLS_BIN)/mockery-$(subst .,-,$(MOCKERY_VERSION))
$(mockery):
	@echo "Installing mockery version $(MOCKERY_VERSION)..."
	@MAJOR=$$(echo "$(MOCKERY_VERSION)" | sed 's/^v\([0-9]*\).*/\1/'); \
	GOBIN=$(TOOLS_BIN) go install "github.com/vektra/mockery/v$${MAJOR}@$(MOCKERY_VERSION)"
	mv $(TOOLS_BIN)/mockery ${mockery}

.PHONY: tools-setup
tools-setup: ## Установить все инструменты разработки
	$(print-tools-header)
	@$(MAKE) --no-print-directory _tools-do-setup

.PHONY: _tools-do-setup
_tools-do-setup: | _tools-do-clean $(golangci-lint) $(scratch) $(mockery)
	@echo "Tools installed successfully"

.PHONY: _tools-do-clean
_tools-do-clean:
	@rm -rf $(TOOLS_BIN)

.PHONY: tools-version
tools-version: ## Показать версии установленных инструментов
	$(print-tools-header)
	@printf "  \033[1m%-22s %-15s %s\033[0m\n" "Tool" "Version" "Status"
	@printf "  %-22s %-15s %s\n" "──────────────────────" "───────────────" "──────────────"
	@printf "  %-22s %-15s " "golangci-lint" "$(GOLANGCI_VERSION)"; \
		if [ -x "$(golangci-lint)" ]; then printf "\033[32m✓ installed\033[0m\n"; else printf "\033[31m✗ not installed\033[0m\n"; fi
	@printf "  %-22s %-15s " "scratch" "$(SCRATCH_VERSION)"; \
		if [ -x "$(scratch)" ]; then printf "\033[32m✓ installed\033[0m\n"; else printf "\033[31m✗ not installed\033[0m\n"; fi
	@printf "  %-22s %-15s " "mockery" "$(MOCKERY_VERSION)"; \
		if [ -x "$(mockery)" ]; then printf "\033[32m✓ installed\033[0m\n"; else printf "\033[31m✗ not installed\033[0m\n"; fi
	@echo ""

.PHONY: tools-update
tools-update: ## Проверить обновления и установить последние версии инструментов
	$(print-tools-header)
	@echo "Checking for updates..."; \
	echo ""; \
	MAKEFILE="$(realpath $(firstword $(MAKEFILE_LIST)))"; \
	UPDATED=0; \
	fetch_latest() { \
		local tag; \
		tag=$$(curl -sL "https://api.github.com/repos/$$1/releases/latest" 2>/dev/null \
			| sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1); \
		if [ -z "$$tag" ] && command -v gh >/dev/null 2>&1; then \
			tag=$$(gh api "repos/$$1/releases/latest" --jq '.tag_name' 2>/dev/null); \
		fi; \
		echo "$$tag"; \
	}; \
	check_update() { \
		local name="$$1" var="$$2" current="$$3" latest="$$4"; \
		if [ -z "$$latest" ]; then \
			printf "  \033[33m⚠\033[0m  %-20s %-15s → \033[33mfailed to fetch\033[0m\n" "$$name" "$$current"; \
		elif [ "$$current" = "$$latest" ]; then \
			printf "  \033[32m✓\033[0m  %-20s %s \033[32m(up to date)\033[0m\n" "$$name" "$$current"; \
		else \
			printf "  \033[36m↑\033[0m  %-20s %-15s → \033[36m%s\033[0m\n" "$$name" "$$current" "$$latest"; \
			sed -i "s/^$$var ?= .*/$$var ?= $$latest/" "$$MAKEFILE"; \
			UPDATED=1; \
		fi; \
	}; \
	\
	LATEST=$$(fetch_latest "golangci/golangci-lint"); \
	check_update "golangci-lint" "GOLANGCI_VERSION" "$(GOLANGCI_VERSION)" "$$LATEST"; \
	\
	LATEST=$$(fetch_latest "webdelo/scratch"); \
	check_update "scratch" "SCRATCH_VERSION" "$(SCRATCH_VERSION)" "$$LATEST"; \
	\
	LATEST=$$(fetch_latest "vektra/mockery"); \
	check_update "mockery" "MOCKERY_VERSION" "$(MOCKERY_VERSION)" "$$LATEST"; \
	\
	echo ""; \
	if [ "$$UPDATED" = "1" ]; then \
		echo "Versions updated in Makefile. Running tools-setup..."; \
		echo ""; \
		$(MAKE) tools-setup; \
	else \
		echo "All tools are up to date. No changes needed."; \
	fi
