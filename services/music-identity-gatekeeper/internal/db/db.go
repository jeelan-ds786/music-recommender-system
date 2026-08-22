package db

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib" // Postgres driver
	"github.com/redis/go-redis/v9"
)

func ConnectPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

func ConnectRedis(addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	_, err := rdb.Ping(context.Background()).Result()
	return rdb, err
}