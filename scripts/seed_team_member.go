//go:build ignore

// Dev bootstrap utility: seed a team_members row + workspace assignment so the
// /team/permissions/me endpoint returns a permission matrix instead of 403.
//
// Usage:
//
//	go run scripts/seed_team_member.go \
//	  --email=arief.faltah@dealls.com \
//	  --role="Super Admin" \
//	  --workspace=75f91966-1a19-4ef4-bfa2-9d553091c92f \
//	  --name="Arief Faltah"
//
// Idempotent: re-running with same email/workspace upserts the assignment.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/Sejutacita/cs-agent-bot/config"
)

func main() {
	email := flag.String("email", "", "user email (required)")
	role := flag.String("role", "Super Admin", "role name from roles table")
	workspaceID := flag.String("workspace", "", "workspace UUID to assign (required)")
	name := flag.String("name", "", "display name (defaults to email local-part)")
	flag.Parse()

	if *email == "" || *workspaceID == "" {
		log.Fatal("--email and --workspace are required")
	}
	if *name == "" {
		*name = strings.SplitN(*email, "@", 2)[0]
	}

	_ = godotenv.Load(".env")
	cfg := config.LoadConfig()

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	var roleID string
	if err := tx.QueryRow(`SELECT id FROM roles WHERE name = $1`, *role).Scan(&roleID); err != nil {
		log.Fatalf("role %q not found: %v", *role, err)
	}

	var workspaceName string
	if err := tx.QueryRow(`SELECT name FROM workspaces WHERE id = $1`, *workspaceID).Scan(&workspaceName); err != nil {
		log.Fatalf("workspace %s not found: %v", *workspaceID, err)
	}

	initials := computeInitials(*name)

	var memberID string
	err = tx.QueryRow(`
		INSERT INTO team_members (name, email, initials, role_id, status, department, avatar_color)
		VALUES ($1, LOWER($2), $3, $4, 'active', 'Engineering', '#3B82F6')
		ON CONFLICT (email) DO UPDATE
		  SET name = EXCLUDED.name,
		      role_id = EXCLUDED.role_id,
		      status = 'active',
		      updated_at = NOW()
		RETURNING id
	`, *name, *email, initials, roleID).Scan(&memberID)
	if err != nil {
		log.Fatalf("upsert team_member: %v", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO member_workspace_assignments (member_id, workspace_id)
		VALUES ($1, $2)
		ON CONFLICT (member_id, workspace_id) DO NOTHING
	`, memberID, *workspaceID); err != nil {
		log.Fatalf("upsert assignment: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Printf("✅ team_member.id=%s\n   email=%s\n   role=%s (id=%s)\n   workspace=%s (%s)\n",
		memberID, *email, *role, roleID, *workspaceID, workspaceName)
}

func computeInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "??"
	}
	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:min(2, len(parts[0]))])
	}
	return strings.ToUpper(string(parts[0][0]) + string(parts[1][0]))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
