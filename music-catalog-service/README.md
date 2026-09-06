# Music Catalog Service

The catalog service exposes public read endpoints and protects operator and ETL writes with the `X-Admin-Key` header.

Copy `.env.example` to `.env`, start `catalog-postgres`, and run:

```sh
go run ./cmd/server
```

Health endpoints are available at `GET /health/live` and `GET /health/ready`.