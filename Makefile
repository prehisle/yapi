.PHONY: lint test build verify

lint:
	golangci-lint run ./...

test:
	go test ./...

build:
	go build ./cmd/gateway

verify:
	START_COMPOSE=true ./scripts/verify_gateway.sh
