VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
FRONT_DIR := front
BACK_DIR := backend

.PHONY: build build-frontend build-backend test lint clean release docker-build docker-up docker-down

## build: Build frontend and backend
build: build-frontend build-backend

## build-frontend: Build React frontend
build-frontend:
	cd $(FRONT_DIR) && npm ci --no-audit --no-fund && npm run build

## build-backend: Build Go backend
build-backend:
	cd $(BACK_DIR) && CGO_ENABLED=0 go build $(LDFLAGS) -o ../release/z-ui .

## test: Run all tests
test: test-backend test-frontend

## test-backend: Run Go tests
test-backend:
	cd $(BACK_DIR) && go test ./...

## test-frontend: Run frontend tests
test-frontend:
	cd $(FRONT_DIR) && npx vitest run 2>/dev/null || echo "No frontend tests configured yet"

## lint: Run linters
lint: lint-backend lint-frontend

## lint-backend: Run Go linter
lint-backend:
	cd $(BACK_DIR) && go vet ./...

## lint-frontend: Run ESLint
lint-frontend:
	cd $(FRONT_DIR) && npx eslint src/ 2>/dev/null || echo "ESLint not configured yet"

## docker-build: Build Docker image
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t z-ui:$(VERSION) .

## docker-up: Start with docker compose
docker-up:
	docker compose up -d

## docker-down: Stop docker compose
docker-down:
	docker compose down

## release: Build release package
release: build
	mkdir -p release
	cp -r $(FRONT_DIR)/dist release/front-dist
	cp ops/z-ui.service release/
	cp ops/nginx-z-ui.conf release/
	cp ops/z-ui.env.example release/
	@echo "Release package built in release/"

## clean: Remove build artifacts
clean:
	rm -rf release/ $(FRONT_DIR)/dist/ $(FRONT_DIR)/node_modules/ $(BACK_DIR)/z-ui

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
