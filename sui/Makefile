test:
	@for move_toml in $(shell find packages -name Move.toml); do \
		dir=$$(dirname "$$move_toml"); \
		echo "Running tests in $$dir..."; \
		pushd "$$dir" && sui move test --skip-fetch-latest-git-deps -d; popd; \
	done

test-ts:
	cd ts && npm test

test-integration:
	./scripts/test-deployment.sh

test-build:
	./scripts/test-build-only.sh

test-docker:
	docker-compose -f docker/docker-compose.test.yml up --build --abort-on-container-exit

start-local:
	./scripts/start-local-network.sh

start-local-simple:
	./scripts/start-local-simple.sh

stop-local:
	pkill -f "sui start" || true
	rm -rf .sui || true
	rm -rf .sui-simple || true
	rm -f test-config.json || true
	rm -f local-network.json || true

test-network:
	./scripts/test-network.sh

build-packages:
	@for move_toml in $(shell find packages -name Move.toml); do \
		dir=$$(dirname "$$move_toml"); \
		echo "Building package in $$dir..."; \
		pushd "$$dir" && sui move build --skip-fetch-latest-git-deps -d; popd; \
	done

clean:
	@for move_toml in $(shell find packages -name Move.toml); do \
		dir=$$(dirname "$$move_toml"); \
		echo "Cleaning $$dir..."; \
		rm -rf "$$dir/build" || true; \
	done
	rm -rf ts/dist || true
	rm -rf .sui || true
	rm -f test-config.json || true
	rm -f deployment-test.json || true

help:
	@echo "Available targets:"
	@echo "  test             - Run Move contract tests"
	@echo "  test-ts          - Run TypeScript SDK tests"
	@echo "  test-build       - Test build infrastructure (recommended)"
	@echo "  test-integration - Run integration deployment tests"
	@echo "  test-docker      - Run tests in Docker environment"
	@echo "  start-local      - Start local Sui network (full featured)"
	@echo "  start-local-simple - Start simple local Sui network"
	@echo "  test-network     - Test running local network functionality"
	@echo "  stop-local       - Stop local Sui network and cleanup"
	@echo "  build-packages   - Build all Move packages"
	@echo "  clean            - Clean build artifacts"
	@echo "  help             - Show this help message"

.PHONY: test test-ts test-build test-integration test-docker start-local start-local-simple test-network stop-local build-packages clean help
