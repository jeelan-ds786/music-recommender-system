FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o identity-svc ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/identity-svc /identity-svc
ENTRYPOINT ["/identity-svc"]