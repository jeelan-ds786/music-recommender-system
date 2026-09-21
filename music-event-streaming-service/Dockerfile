FROM golang:1.26.6-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o streaming-svc ./cmd/server

FROM alpine:3.22
RUN addgroup -S streaming && adduser -S streaming -G streaming
COPY --from=builder /app/streaming-svc /streaming-svc
USER streaming
ENTRYPOINT ["/streaming-svc"]