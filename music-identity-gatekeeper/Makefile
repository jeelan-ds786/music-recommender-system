include .env
export

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1

dev:
	go run ./cmd/server

test:
	go test ./internal/preference/... -v

