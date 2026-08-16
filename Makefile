.PHONY: server-run agent-run web-build test

server-run:
	cd server && go run ./cmd/serverwatch

agent-run:
	cd agent && go run ./cmd/serverwatch-agent

web-build:
	cd web && pnpm build

test:
	cd server && go test ./...
	cd agent && go test ./...
