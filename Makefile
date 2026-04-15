.PHONY: build genplugins run test clean docker-up docker-down

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