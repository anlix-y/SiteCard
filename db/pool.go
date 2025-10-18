package db

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

var Pool *pgxpool.Pool

func InitPool() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:2323@localhost:5432/go_test_post"
	}
	p, err := pgxpool.New(context.Background(), url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect error: %v\n", err)
		os.Exit(1)
	}
	Pool = p
}

func ClosePool() {
	if Pool != nil {
		Pool.Close()
	}
}
