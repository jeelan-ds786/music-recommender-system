FROM golang:1.26.6-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o catalog-svc ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/catalog-svc /catalog-svc
ENTRYPOINT ["/catalog-svc"]