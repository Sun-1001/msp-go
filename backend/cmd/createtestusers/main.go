package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/math_platform?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	users := []struct {
		username string
		role     string
	}{
		{"student1", "STUDENT"},
		{"student2", "STUDENT"},
		{"teacher1", "TEACHER"},
		{"teacher2", "TEACHER"},
	}

	for _, u := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hash: %v\n", err)
			os.Exit(1)
		}

		id := fmt.Sprintf("test-%s", u.username)
		_, err = pool.Exec(ctx, `
			INSERT INTO public.users (id, username, email, hashed_password, role, display_name, is_active, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5::public.userrole, $6, true, 'ACTIVE'::public.userstatus, now(), now())
			ON CONFLICT (username) DO NOTHING`,
			id, u.username, u.username+"@test.local", string(hash), u.role, u.username,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create %s: %v\n", u.username, err)
		} else {
			fmt.Printf("created: %s (role: %s, password: 123456)\n", u.username, u.role)
		}
	}
}
