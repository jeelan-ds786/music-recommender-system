FROM golang:1.26.6-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o playback-svc ./cmd/server

FROM alpine:3.22
RUN addgroup -S playback && adduser -S playback -G playback
COPY --from=builder /app/playback-svc /playback-svc
USER playback
ENTRYPOINT ["/playback-svc"]