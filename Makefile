.PHONY: generate
generate:
	go tool oapi-codegen \
		-generate types,chi-server,spec \
		-package api \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml
	go generate ./...

.PHONY: run
run:
	export $$(grep -v '^#' .env | xargs) && go run ./cmd/trip-service/main.go

.PHONY: migrate
migrate:
	goose -dir migrations postgres "postgres://tripgo:tripgo@localhost:22032/tripgo?sslmode=disable" up

.PHONY: test
test:
	go test -v ./...

.PHONY: test-integration
test-integration:
	go test -v -tags=integration ./...

.PHONY: lint
lint:
	golangci-lint run