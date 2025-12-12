package db

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func InitPool() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:2323@localhost:5432/testcard"
	}
	p, err := pgxpool.New(context.Background(), url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect error: %v\n", err)
		os.Exit(1)
	}
	Pool = p

	// Ensure minimal required tables exist and optional columns
	_, _ = Pool.Exec(context.Background(), `
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    email TEXT
);

-- Таблица постов (контент), теперь поддерживает владельца (user_id)
CREATE TABLE IF NOT EXISTS post (
    id SERIAL PRIMARY KEY,
    label TEXT NOT NULL,
    text  TEXT NOT NULL,
    user_id INT NULL,
    CONSTRAINT fk_post_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS settings (
    user_id INT UNIQUE NOT NULL,
    home_bg_url TEXT,
    link_github TEXT,
    link_tg TEXT,
    link_custom TEXT,
    slug TEXT,
    CONSTRAINT fk_settings_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS settings_slug_uq ON settings (slug) WHERE slug IS NOT NULL;

-- Таблица проектов (GitHub репозитории)
CREATE TABLE IF NOT EXISTS projects (
    id SERIAL PRIMARY KEY,
    repo_name TEXT UNIQUE NOT NULL,
    title TEXT,
    description TEXT,
    image_url TEXT,
    github_url TEXT,
    custom_url TEXT,
    enabled BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS github_tokens (
    id SERIAL PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMP NULL,
    fail_count INT NOT NULL DEFAULT 0
);
    `)

	// Backward-compatible ALTERs for existing databases created ранее без email/role
	_, _ = Pool.Exec(context.Background(), `
ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT;
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'user';
UPDATE users SET role = 'user' WHERE role IS NULL;
ALTER TABLE users ALTER COLUMN role SET NOT NULL;
`)

	// Ensure post.user_id exists in legacy databases
	_, _ = Pool.Exec(context.Background(), `
ALTER TABLE post ADD COLUMN IF NOT EXISTS user_id INT NULL;
ALTER TABLE post ADD CONSTRAINT IF NOT EXISTS fk_post_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
`)

	// Create indexes that could fail earlier if column didn't exist
	_, _ = Pool.Exec(context.Background(), `
CREATE UNIQUE INDEX IF NOT EXISTS users_email_uq ON users (email) WHERE email IS NOT NULL AND email <> '';
`)

	// Seed tokens from env GITHUB_TOKENS (comma separated)
	if toks := os.Getenv("GITHUB_TOKENS"); toks != "" {
		for _, t := range strings.Split(toks, ",") {
			token := strings.TrimSpace(t)
			if token == "" {
				continue
			}
			_, _ = Pool.Exec(context.Background(), `
                INSERT INTO github_tokens(token) VALUES($1)
                ON CONFLICT(token) DO NOTHING
            `, token)
		}
	}
}

func ClosePool() {
	if Pool != nil {
		Pool.Close()
	}
}
