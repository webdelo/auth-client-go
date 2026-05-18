# dev/deployments/extra.mk
#
# Integration test environment management for auth-client-go SDK.
# Included from the main Makefile via `-include dev/deployments/extra.mk`.

# docker compose с указанием compose файла и env-файла
_COMPOSE := docker compose \
    -f dev/deployments/compose.test.yaml \
    --env-file dev/deployments/test.env

.PHONY: dev-up-test-env dev-down-test-env env-up env-down test-integration

##@ Integration Testing

dev-up-test-env: ## Bring up AUTH test environment (postgres + migrations + auth-app), wait until healthy
	@echo "━━━ Starting AUTH test environment ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	${_COMPOSE} --profile test up -d --wait auth-postgres
	@echo "Running AUTH migrations..."
	${_COMPOSE} --profile test run --rm auth-migrate
	@echo "Starting AUTH server (waiting for healthy)..."
	${_COMPOSE} --profile test up -d --wait --no-deps auth-app
	@echo "✅ AUTH test environment ready (gRPC on localhost:50051)"

dev-down-test-env: ## Tear down AUTH test environment and delete volumes
	@echo "━━━ Stopping AUTH test environment ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	${_COMPOSE} --profile test down -v --remove-orphans
	@echo "✅ AUTH test environment stopped"

env-up: dev-up-test-env ## Alias for dev-up-test-env

env-down: dev-down-test-env ## Alias for dev-down-test-env

test-integration: $(TEST_RUNNER) dev-up-test-env ## Run integration tests against real AUTH (auto env up/down)
	@echo "━━━ Integration Tests ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@set +e; \
		RUN_INTEGRATION_TESTS=true AUTH_ENDPOINT=localhost:50051 \
		$(TEST_RUNNER) -tags integration --title="Integration Tests" ./lib/...; \
		TEST_EXIT=$$?; \
		echo ""; \
		$(MAKE) --no-print-directory dev-down-test-env; \
		exit $$TEST_EXIT
