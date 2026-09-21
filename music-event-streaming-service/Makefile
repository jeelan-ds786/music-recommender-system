-include .env
export

.PHONY: fmt test check migrate-up migrate-down proto-gen dev

fmt:
	gofmt -w .

test:
	go test -race ./...

check:
	test -z "$$(gofmt -l .)"
	go mod tidy -diff
	go mod verify
	go vet ./...
	go build ./cmd/server
	go test -race ./...

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down -all

proto-gen:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	protoc --plugin=protoc-gen-go="$$(go env GOPATH)/bin/protoc-gen-go" -I proto --go_out=. --go_opt=module=github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service proto/playback/v1/events.proto proto/identity/v1/events.proto proto/recommendation/v1/feedback.proto proto/streaming/v1/envelopes.proto

dev:
	go run ./cmd/server