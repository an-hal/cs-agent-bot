package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/Sejutacita/cs-agent-bot/internal/migration"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load(".env")

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/migrate.go [up|down|create <name>]")
	}

	command := os.Args[1]
	dir := "migration"

	// Handle create command (doesn't need DB connection)
	if command == "create" {
		if len(os.Args) < 3 {
			log.Fatal("Usage: go run cmd/migrate.go create <name>\nExample: go run cmd/migrate.go create add_payments_table")
		}
		name := os.Args[2]
		if err := migration.CreateMigration(dir, name); err != nil {
			log.Fatalf("❌ Failed to create migration: %v", err)
		}
		return
	}

	// For up/down commands, we need DB connection
	db := connectDB()
	defer db.Close()

	switch command {
	case "up":
		migration.MigrateUp(db, dir)
	case "down":
		migration.MigrateDown(db, dir)
	default:
		log.Fatalf("Unknown command: %s\nUsage: go run cmd/migrate.go [up|down|create <name>]", command)
	}
}

func connectDB() *sql.DB {
	// Read config from env
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbSSLMode := os.Getenv("DB_SSLMODE")

	if dbHost == "" || dbUser == "" || dbPassword == "" || dbName == "" {
		log.Fatal("Missing required environment variables (DB_HOST, DB_USER, DB_PASSWORD, DB_NAME)")
	}

	dsn := "host=" + dbHost +
		" port=" + dbPort +
		" user=" + dbUser +
		" password=" + dbPassword +
		" dbname=" + dbName +
		" sslmode=" + dbSSLMode

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	return db
}
