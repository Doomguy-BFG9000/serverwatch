# ServerWatch Community Edition - top-level build entry points.
#
# Quick start: `make help`.
#
# VERSION defaults to the contents of VERSION (without the leading "v") or
# "dev". `make release` writes the git tag to VERSION. Build metadata is
# injected into both the Go binary and the web bundle so the dashboard,
# GET /health, and the binary all report the same build.

VERSION   ?= $(shell cat VERSION 2>/dev/null | tr -d '[:space:]' || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w \
             -X github.com/serverwatch/serverwatch/server/internal/version.Version=$(VERSION) \
             -X github.com/serverwatch/serverwatch/server/internal/version.Commit=$(COMMIT) \
             -X github.com/serverwatch/serverwatch/server/internal/version.Date=$(DATE)

.PHONY: help server-run agent-run web-build build test fmt vet lint \
        test-server test-agent test-web check release clean

help:
	@echo "ServerWatch Community Edition (VERSION=$(VERSION), COMMIT=$(COMMIT))"
	@echo ""
	@echo "  make build        Build the backend binary into bin/serverwatch"
	@echo "  make web-build    Build the dashboard into web/dist"
	@echo "  make test         Run all Go and web tests"
	@echo "  make check        fmt + vet + test + build + web-build (what CI runs)"
	@echo "  make release VERSION=v0.1.0   Build a release bundle into dist/"
	@echo "  make clean        Remove build outputs"
	@echo ""
	@echo "  make server-run   Run the backend locally"
	@echo "  make agent-run    Run the agent locally"

server-run:
	cd server && go run ./cmd/serverwatch

agent-run:
	cd agent && go run ./cmd/serverwatch-agent

# Version is baked into the bundle so the dashboard badge matches /health.
web-build:
	cd web && VITE_APP_VERSION=$(VERSION) pnpm build

build:
	cd server && go build -ldflags "$(LDFLAGS)" -o ../bin/serverwatch ./cmd/serverwatch

test: test-server test-agent test-web

test-server:
	cd server && go test ./...

test-agent:
	cd agent && go test ./...

test-web:
	cd web && pnpm lint && VITE_APP_VERSION=$(VERSION) pnpm exec tsc -b

fmt:
	cd server && gofmt -w .
	cd agent && gofmt -w .

vet:
	cd server && go vet ./...
	cd agent && go vet ./...

lint: vet test-web

# The full gate used by CI and before a release.
check: vet test build web-build

release:
	./scripts/release.sh $(VERSION)

clean:
	rm -rf bin dist web/dist web/node_modules/.tmp
