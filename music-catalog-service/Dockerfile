FROM golang:1.26.6-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o catalog-svc ./cmd/server

FROM alpine:3.22
RUN addgroup -S catalog && adduser -S catalog -G catalog
COPY --from=builder /app/catalog-svc /catalog-svc
USER catalog
ENTRYPOINT ["/catalog-svc"]