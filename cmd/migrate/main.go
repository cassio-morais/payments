package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		direction string
		dbURL     string
		path      string
	)

	flag.StringVar(&direction, "direction", "up", "Migration direction: up or down")
	flag.StringVar(&dbURL, "db", "", "Database URL (or set DATABASE_URL env var)")
	flag.StringVar(&path, "path", "internal/infrastructure/postgres/migrations", "Path to migration files")
	flag.Parse()

	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		dbURL = "postgresql://payments:payments@localhost:5432/payments?sslmode=disable"
	}

	m, err := migrate.New("file://"+path, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create migrate instance: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	switch direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			fmt.Fprintf(os.Stderr, "Migration up failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations applied successfully")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			fmt.Fprintf(os.Stderr, "Migration down failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations rolled back successfully")
	default:
		fmt.Fprintf(os.Stderr, "Unknown direction: %s (use 'up' or 'down')\n", direction)
		os.Exit(1)
	}
}
