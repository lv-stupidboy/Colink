.PHONY: build genplugins run test clean docker-up docker-down sync-resources release desktop-dev desktop-build desktop-package desktop-package-all test-all test-frontend test-backend test-performance test-feature test-feature-priority test-p0 test-p1

# 从 VERSION 文件读取版本号
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y%m%d-%H%M%S 2>/dev/null || echo "unknown")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
FULL_VERSION := v$(VERSION)-$(BUILD_TIME)

# Auto-discover plugins before build
genplugins:
	@echo "Generating plugin registry..."
	@go run ./tools/genplugins

build: genplugins
	go build -ldflags "-X main.Version=$(FULL_VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" -o bin/isdp-server.exe ./cmd/server

# 同步资源到 installer-tauri
sync-resources:
	@echo "Syncing resources to installer-tauri..."
	node scripts/sync-resources.js

# 完整发布构建（后端 + 前端 + 同步 + 安装程序）
release: build
	@echo "Building frontend..."
	cd web && npm run build
	@echo "Syncing resources..."
	node scripts/sync-resources.js
	@echo "Building installer..."
	cd installer-tauri && ./build-tauri.ps1

run:
	go run ./cmd/server

test:
	go test ./... -v -cover

clean:
	rm -rf bin/

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# ===== Auto-Test 测试命令 =====

test-all: test-backend test-frontend test-performance

test-frontend:
	cd web && npx playwright test auto-test/e2e/
	cd web && npx vitest run auto-test/vitest/

test-backend:
	go test ./auto-test/internal/... -v

test-performance:
	go test -bench=. ./auto-test/internal/performance/
	cd web && npx playwright test --trace on auto-test/e2e/performance/

test-feature:
	@if [ -z "$(F)" ]; then \
		echo "请指定特性 ID，例如: make test-feature F=F001"; \
		exit 1; \
	fi
	@echo "执行特性测试: $(F)"
	@python scripts/run-feature-tests.py --feature $(F)

test-feature-priority:
	@if [ -z "$(P)" ]; then \
		echo "请指定优先级，例如: make test-feature-priority P=P0"; \
		exit 1; \
	fi
	@echo "执行优先级特性测试: $(P)"
	@python scripts/run-feature-tests.py --priority $(P)

test-p0:
	go test ./auto-test/internal/... -v -run "P0"
	cd web && npx playwright test auto-test/e2e/ --grep "P0"

test-p1:
	go test ./auto-test/internal/... -v -run "P0|P1"
	cd web && npx playwright test auto-test/e2e/ --grep "P0|P1"

# ===== Desktop application build =====

desktop-dev:
	cd apps/desktop && npm run dev

desktop-build:
	cd apps/desktop && npm run build

desktop-package:
	cd apps/desktop && npm run package

desktop-package-all:
	cd apps/desktop && npm run package:all
