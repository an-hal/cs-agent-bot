package migration

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// readSQLFile reads a file and returns its content as string
func readSQLFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return string(content), nil
}

// runSQL executes raw SQL from a file
func runSQL(db *sql.DB, path string) {
	sqlContent, err := readSQLFile(path)
	if err != nil {
		log.Fatalf("❌ Error reading SQL: %v", err)
	}

	_, err = db.Exec(sqlContent)
	if err != nil {
		log.Fatalf("❌ Error executing SQL in %s: %v", path, err)
	}
}

// extractVersion extracts the version (timestamp) from a file name
// Example: "20250725100000_add_books_table.up.sql" => "20250725100000"
func extractVersion(filePath string) string {
	base := filepath.Base(filePath)
	parts := strings.Split(base, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// getAppliedVersions reads versions from schema_migrations table
func getAppliedVersions(db *sql.DB) map[string]bool {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
        version TEXT PRIMARY KEY,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`)
	if err != nil {
		log.Fatalf("❌ Failed to ensure schema_migrations table: %v", err)
	}

	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		log.Fatalf("❌ Failed to query schema_migrations: %v", err)
	}
	defer rows.Close()

	versions := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			log.Fatalf("❌ Failed to scan migration version: %v", err)
		}
		versions[version] = true
	}

	return versions
}

// ValidateMigrationName checks if name contains only alphanumeric and underscores
func ValidateMigrationName(name string) error {
	if name == "" {
		return fmt.Errorf("migration name cannot be empty")
	}

	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, name)
	if !matched {
		return fmt.Errorf("migration name must contain only alphanumeric characters and underscores")
	}
	return nil
}

// CreateMigration generates new migration files with timestamp prefix
func CreateMigration(dir string, name string) error {
	if err := ValidateMigrationName(name); err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create migration directory: %w", err)
	}

	timestamp := time.Now().Format("20060102150405")

	upFile := filepath.Join(dir, fmt.Sprintf("%s_%s.up.sql", timestamp, name))
	downFile := filepath.Join(dir, fmt.Sprintf("%s_%s.down.sql", timestamp, name))

	upContent := fmt.Sprintf(`-- Migration: %s
-- Created at: %s

-- Write your UP migration SQL here

`, name, time.Now().Format(time.RFC3339))

	downContent := fmt.Sprintf(`-- Migration: %s (rollback)
-- Created at: %s

-- Write your DOWN migration SQL here (reverse the UP migration)

`, name, time.Now().Format(time.RFC3339))

	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		return fmt.Errorf("failed to create up migration: %w", err)
	}

	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		os.Remove(upFile)
		return fmt.Errorf("failed to create down migration: %w", err)
	}

	fmt.Printf("✅ Created migration files:\n")
	fmt.Printf("   📄 %s\n", upFile)
	fmt.Printf("   📄 %s\n", downFile)

	return nil
}
