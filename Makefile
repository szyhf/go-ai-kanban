.PHONY: build run test clean tidy fmt lint

# Binary name
BINARY=ai-kanban-go

# Build flags
LDFLAGS=-s -w

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/server

run:
	go run ./cmd/server

test:
	go test -race -cover ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .
	goimports -w .

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

# Build with frontend embedded
build-release: frontend
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/server

frontend:
	cd packages/local-web && pnpm install && pnpm run build
	rm -rf web/dist
	cp -r packages/local-web/dist web/dist

# Development with hot reload (requires air)
dev:
	air -c .air.toml

# Generate Go struct from SQL migrations (helper)
gen-models:
	go run ./cmd/gen-models

# Docker build
docker:
	docker build -t ai-kanban-go .
