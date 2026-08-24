include .env
export

.PHONY: fmt lint lint-fix test check migrate-up migrate-down

fmt:
	gofmt -w .

lint:
	golangci-lint run ./...

lint-fix:
	gofmt -w .
	golangci-lint run --fix ./...

test:
	go test -race ./...

check:
	test -z "$$(gofmt -l .)"
	go mod tidy -diff
	go mod verify
	golangci-lint run ./...
	go vet ./...
	go build ./cmd/api ./cmd/server
	go test -race ./...

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1