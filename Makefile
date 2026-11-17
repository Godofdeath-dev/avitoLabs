SHELL := bash

.PHONY: generate run migrate test test-integration lint

generate:
	go tool oapi-codegen -generate types,chi-server,spec -package api -o internal/generated/api.gen.go contracts/openapi/trip-service.openapi.yaml
	go generate ./...

run:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run ./cmd/trip-service

migrate:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" up

test:
	go test -race -count=2 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-integration:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go test -race -count=2 -tags=integration ./...

lint:
	golangci-lint run
