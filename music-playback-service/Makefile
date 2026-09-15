-include .env
export

.PHONY: fmt lint lint-fix test check migrate-up migrate-down migrate-down-clean proto-gen dev

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
	go build ./cmd/server
	go test -race ./...

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1

migrate-down-clean:
	migrate -path migrations -database "$(DB_URL)" down -all

proto-gen:
	protoc -I proto --go_out=. --go_opt=module=github.com/jeelan-ds786/music-recommender-system/music-playback-service proto/playback/v1/events.proto

dev:
	go run ./cmd/server
