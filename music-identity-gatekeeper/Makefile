include .env
export

.PHONY: fmt lint lint-fix test check migrate-up migrate-down migrate-down-clean proto-gen

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

migrate-down-clean:
	migrate -path migrations -database "$(DB_URL)" down -all

# Requires protoc (https://github.com/protocolbuffers/protobuf/releases)
# and protoc-gen-go (go install google.golang.org/protobuf/cmd/protoc-gen-go@latest)
# on PATH.
proto-gen:
	protoc \
		-I proto \
		--go_out=. \
		--go_opt=module=github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper \
		proto/identity/v1/events.proto

dev:
	go run ./cmd/server

test:
	go test ./internal/preference/... ./internal/profile/... ./internal/event/... -v

