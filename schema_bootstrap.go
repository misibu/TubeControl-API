package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// init prepares a private application schema before main creates its pool.
// Timeweb can grant CREATE on the database while denying CREATE on the shared
// public schema. Using our own schema avoids changing permissions on public.
func init() {
	baseDSN := databaseURL()
	if strings.TrimSpace(baseDSN) == "" {
		return
	}

	bootstrapDSN := baseDSN
	if u, err := url.Parse(baseDSN); err == nil {
		q := u.Query()
		q.Del("search_path")
		u.RawQuery = q.Encode()
		bootstrapDSN = u.String()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, bootstrapDSN)
	if err != nil {
		log.Printf("PostgreSQL schema bootstrap connection failed: %v", err)
		return
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS tubecontrol`); err != nil {
		log.Printf("PostgreSQL schema bootstrap failed: %v", err)
		return
	}

	u, err := url.Parse(baseDSN)
	if err != nil {
		log.Printf("PostgreSQL schema DSN parse failed: %v", err)
		return
	}
	q := u.Query()
	q.Set("search_path", "tubecontrol")
	u.RawQuery = q.Encode()
	if err := os.Setenv("DATABASE_URL", u.String()); err != nil {
		log.Printf("PostgreSQL schema environment setup failed: %v", err)
		return
	}
	log.Printf("PostgreSQL application schema ready: tubecontrol")
}
