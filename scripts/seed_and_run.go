//go:build ignore

// seed_and_run.go — Seeds test records then triggers the cron runner.
// Behaves like an n8n "manual test run": reset → seed → trigger → show results.
//
// Two execution paths are seeded together:
//
//	Legacy runner  — LEAD-stage clients (SEED-<WS>-L01…L05), uses trigger_rules
//	Workflow engine — CLIENT-stage clients (SEED-<WS>-W01…W04), uses workflow nodes
//	               + automation_rules (requires USE_WORKFLOW_ENGINE=true in .env)
//
// Usage:
//
//	go run scripts/seed_and_run.go
//	go run scripts/seed_and_run.go --workspace=kantorku --server=http://localhost:8080
//
// Requirements:
//   - Server running with ENV=development or ENV=local (OIDC bypass is automatic)
//   - .env present (or env vars set)
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/Sejutacita/cs-agent-bot/config"
)

// ── Scenario definitions ──────────────────────────────────────────────────────

// legacyScenario seeds LEAD-stage clients for the P0–P5 legacy trigger runner.
// nps/usage/planType go into custom_fields JSONB (columns dropped in 20260427000200).
type legacyScenario struct {
	n         int
	label     string
	payStatus string
	nps       int
	usage     int
	planType  string
	cStartOff int
	cEndOff   int
}

var legacyScenarios = []legacyScenario{
	{1, "Healthy Paid", "Paid", 9, 85, "Enterprise", -90, 275},
	{2, "Low NPS Risk", "Paid", 3, 70, "Pro", -120, 245},
	{3, "Renewal H-45", "Paid", 7, 75, "Pro", -320, 45},
	{4, "Invoice PRE7", "Pending", 7, 60, "Pro", -358, 7},
	{5, "Overdue POST4", "Overdue", 5, 50, "Basic", -369, -4},
}

// workflowScenario seeds CLIENT-stage clients for the workflow engine path.
// nps/usage/planType go into custom_fields JSONB — conditions reference them via
// GetField which falls through to custom_fields when the key is not a core field.
type workflowScenario struct {
	n           int
	label       string
	payStatus   string
	riskFlag    string // risk_flag_text — direct column, still exists
	nps         int    // custom_fields.nps_score
	usage       int    // custom_fields.usage_score
	planType    string // custom_fields.plan_type
	cStartOff   int
	cEndOff     int // also written to days_to_expiry column
	triggerRule string
}

var workflowScenarios = []workflowScenario{
	// W01: nps=3 in custom_fields → TEST_HEALTH_NPS condition: nps_score <= 4
	{1, "Health Risk (low NPS)", "Paid", "None", 3, 40, "Pro", -90, 275, "TEST_HEALTH_NPS"},
	// W02: days_to_expiry=45 → TEST_REN45 (static column)
	{2, "Renewal H-45", "Paid", "None", 8, 75, "Enterprise", -320, 45, "TEST_REN45"},
	// W03: days_to_expiry=7 + Pending → TEST_PRE7
	{3, "Invoice PRE7", "Pending", "None", 7, 60, "Pro", -358, 7, "TEST_PRE7"},
	// W04: days_to_expiry=-4 + Overdue → TEST_OVERDUE
	{4, "Overdue POST4", "Overdue", "None", 5, 50, "Basic", -369, -4, "TEST_OVERDUE"},
}

// ── Node data (n8n-style canvas) ──────────────────────────────────────────────

type nodeData struct {
	Category    string `json:"category"`
	Label       string `json:"label"`
	Icon        string `json:"icon"`
	Description string `json:"description,omitempty"`
	TriggerID   string `json:"trigger_id,omitempty"`
	Condition   string `json:"condition,omitempty"`
	SentFlag    string `json:"sent_flag,omitempty"`
	Color       string `json:"color,omitempty"`
	Bg          string `json:"bg,omitempty"`
}

type wfNode struct {
	nodeID string
	x, y   float32
	data   nodeData
}

// workflowCanvas returns the nodes and edges for the AE test workflow canvas.
// Layout mirrors n8n: trigger at top, condition nodes below, linear chain.
func workflowCanvas() ([]wfNode, [][2]string) {
	nodes := []wfNode{
		{"node-trigger", 200, 0, nodeData{
			Category:    "trigger",
			Label:       "AE: CLIENT Records",
			Icon:        "🏁",
			Description: "Entry point — processes all active CLIENT-stage records",
			Color:       "#6366f1",
		}},
		{"node-health", 200, 160, nodeData{
			Category:  "condition",
			Label:     "Health Risk?",
			Icon:      "⚠️",
			TriggerID: "TEST_HEALTH_NPS",
			Condition: "nps_score <= 4",
			SentFlag:  "test_health_sent",
			Color:     "#ef4444",
			Bg:        "#fef2f2",
		}},
		{"node-renewal", 200, 320, nodeData{
			Category:  "condition",
			Label:     "Renewal H-45?",
			Icon:      "📅",
			TriggerID: "TEST_REN45",
			Condition: "days_to_expiry BETWEEN 0 AND 45",
			SentFlag:  "test_ren45_sent",
			Color:     "#f59e0b",
			Bg:        "#fffbeb",
		}},
		{"node-invoice", 200, 480, nodeData{
			Category:  "condition",
			Label:     "Invoice PRE7?",
			Icon:      "🧾",
			TriggerID: "TEST_PRE7",
			Condition: "days_to_expiry BETWEEN 0 AND 7\nAND payment_status = Pending",
			SentFlag:  "test_pre7_sent",
			Color:     "#3b82f6",
			Bg:        "#eff6ff",
		}},
		{"node-overdue", 200, 640, nodeData{
			Category:  "condition",
			Label:     "Overdue?",
			Icon:      "🚨",
			TriggerID: "TEST_OVERDUE",
			Condition: "days_to_expiry <= -1\nAND payment_status = Overdue",
			SentFlag:  "test_overdue_sent",
			Color:     "#dc2626",
			Bg:        "#fef2f2",
		}},
	}

	edges := [][2]string{
		{"node-trigger", "node-health"},
		{"node-health", "node-renewal"},
		{"node-renewal", "node-invoice"},
		{"node-invoice", "node-overdue"},
	}

	return nodes, edges
}

// ── Automation rules matching the canvas nodes ────────────────────────────────

type automationRule struct {
	ruleCode  string
	phase     string
	condition string
	sentFlag  string
}

var automationRules = []automationRule{
	{
		ruleCode:  "TEST_HEALTH_NPS",
		phase:     "HEALTH",
		condition: "nps_score <= 4",
		sentFlag:  "test_health_sent",
	},
	{
		ruleCode:  "TEST_REN45",
		phase:     "NEGOTIATION",
		condition: "days_to_expiry BETWEEN 0 AND 45",
		sentFlag:  "test_ren45_sent",
	},
	{
		ruleCode:  "TEST_PRE7",
		phase:     "INVOICE",
		condition: "days_to_expiry BETWEEN 0 AND 7\nAND payment_status = Pending",
		sentFlag:  "test_pre7_sent",
	},
	{
		ruleCode:  "TEST_OVERDUE",
		phase:     "OVERDUE",
		condition: "days_to_expiry <= -1\nAND payment_status = Overdue",
		sentFlag:  "test_overdue_sent",
	},
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	wsSlug := flag.String("workspace", "dealls", "workspace slug to seed and run")
	server := flag.String("server", "http://localhost:8080", "server base URL")
	flag.Parse()

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

	// 1. Resolve workspace
	var wsID, wsName string
	if err := db.QueryRow(`SELECT id, name FROM workspaces WHERE slug = $1 AND is_holding = FALSE`, *wsSlug).
		Scan(&wsID, &wsName); err != nil {
		log.Fatalf("workspace %q not found (must be non-holding): %v", *wsSlug, err)
	}
	fmt.Printf("🔍 Workspace    : %s (%s)\n\n", wsName, wsID)

	legacyPrefix := legacyCompanyPrefix(*wsSlug)
	wfPrefix := workflowCompanyPrefix(*wsSlug)

	// 2. Reset: delete old seed clients (flags cascade via FK)
	for _, prefix := range []string{legacyPrefix, wfPrefix} {
		_, _ = db.Exec(`DELETE FROM clients WHERE workspace_id = $1 AND company_id LIKE $2`, wsID, prefix+"%")
	}
	fmt.Printf("🗑  Reset seed clients (%s*, %s*)\n", legacyPrefix, wfPrefix)

	// 3. Seed legacy LEAD-stage clients (legacy P0-P5 runner)
	titled := titledSlug(*wsSlug)
	n := seedLegacyClients(db, wsID, titled, legacyPrefix)
	fmt.Printf("🌱 Legacy  : seeded %d LEAD-stage clients  (%s*)\n", n, legacyPrefix)

	// 4. Seed workflow engine: nodes + automation_rules + CLIENT-stage clients
	seedWorkflowEngine(db, wsID, *wsSlug, titled, wfPrefix)
	fmt.Println()

	// 5. Trigger cron
	fmt.Printf("🚀 Triggering   GET %s/cron/run …\n", *server)
	triggerTime := time.Now()
	resp, err := http.Get(*server + "/cron/run")
	if err != nil {
		log.Fatalf("trigger cron: %v\n  → is the server running at %s?", err, *server)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		log.Fatalf("unexpected HTTP %d:\n%s", resp.StatusCode, body)
	}
	fmt.Printf("✅ Accepted     HTTP %d\n", resp.StatusCode)

	jobID := extractJobID(body, wsID)
	if jobID != "" {
		fmt.Printf("📋 Job ID       : %s\n", jobID)
	}

	// 6. Poll background job
	finalJobID := pollJob(db, wsID, jobID, triggerTime)

	// 7. Show results
	fmt.Println()
	printResults(db, wsID, finalJobID, legacyPrefix, wfPrefix)
}

// ── Seeding helpers ───────────────────────────────────────────────────────────

func legacyCompanyPrefix(wsSlug string) string {
	return "SEED-" + slugUpper(wsSlug) + "-L"
}

func workflowCompanyPrefix(wsSlug string) string {
	return "SEED-" + slugUpper(wsSlug) + "-W"
}

func slugUpper(s string) string {
	if len(s) > 8 {
		s = s[:8]
	}
	return strings.ToUpper(s)
}

func titledSlug(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func seedLegacyClients(db *sql.DB, wsID, titled, prefix string) int {
	const q = `
INSERT INTO clients (
  company_id, company_name, pic_name, pic_wa, pic_email, pic_role,
  owner_name, owner_wa, owner_telegram_id,
  payment_terms,
  contract_start, contract_end, contract_months, activation_date,
  payment_status, blacklisted,
  stage, custom_fields, workspace_id
) VALUES (
  $1, $2, $3, $4, $5, 'HR Manager',
  $6, $7, $8,
  'Net 30',
  CURRENT_DATE + $9::int, CURRENT_DATE + $10::int, 12, CURRENT_DATE + $9::int + 14,
  $11, FALSE,
  'LEAD', $12::jsonb, $13
)
ON CONFLICT (company_id) DO NOTHING`

	count := 0
	for i, s := range legacyScenarios {
		companyID := fmt.Sprintf("%s%02d", prefix, s.n)
		if err := execScenario(db, q, companyID, titled, s.n, i, s.payStatus, s.planType, s.nps, s.usage,
			s.cStartOff, s.cEndOff, wsID); err != nil {
			log.Printf("  ⚠️  insert %s: %v", companyID, err)
		} else {
			count++
		}
	}
	return count
}

func execScenario(db *sql.DB, q, companyID, titled string, n, i int, payStatus, planType string,
	nps, usage, cStartOff, cEndOff int, wsID string) error {
	segment := map[int]string{1: "High", 2: "Mid", 3: "High", 4: "Mid", 5: "Low"}[n]
	_, err := db.Exec(q,
		companyID,
		fmt.Sprintf("Seed %s %02d", titled, n),
		fmt.Sprintf("PIC %s %d", titled, n),
		fmt.Sprintf("+6289%09d", (i+1)*100+n),
		fmt.Sprintf("pic%d@seed-%s.test", n, strings.ToLower(titled)),
		fmt.Sprintf("Owner %s %d", titled, n),
		fmt.Sprintf("+6288%09d", (i+1)*100+n),
		fmt.Sprintf("9%08d", (i+1)*100+n),
		cStartOff, cEndOff,
		payStatus,
		customFieldsJSON(nps, usage, planType, segment),
		wsID,
	)
	return err
}

func seedWorkflowEngine(db *sql.DB, wsID, wsSlug, titled, prefix string) {
	// 4a. Ensure custom_field_definitions exist for this workspace
	seedCustomFieldDefs(db, wsID)

	// 4b. Upsert test workflow
	wfID := upsertWorkflow(db, wsID)
	if wfID == "" {
		fmt.Println("⚠️  Could not create workflow — skipping workflow engine seeding")
		return
	}
	fmt.Printf("🔧 Workflow : id=%s (slug=ae-test)\n", wfID)

	// 4c. Replace workflow nodes and edges
	seedWorkflowNodes(db, wfID)
	seedWorkflowEdges(db, wfID)

	// 4d. Upsert automation rules
	n := seedAutomationRules(db, wsID)
	fmt.Printf("📐 Rules    : seeded/updated %d automation_rules (role=ae)\n", n)

	// 4e. Seed CLIENT-stage clients
	nc := seedWorkflowClients(db, wsID, titled, prefix)
	fmt.Printf("🌿 Workflow : seeded %d CLIENT-stage clients (%s*)\n", nc, prefix)
	fmt.Printf("   💡 Set USE_WORKFLOW_ENGINE=true in .env to activate this path\n")
}

func upsertWorkflow(db *sql.DB, wsID string) string {
	var wfID string
	err := db.QueryRow(`
		INSERT INTO workflows (workspace_id, name, icon, slug, description, status, stage_filter, created_by)
		VALUES ($1, 'Test AE Workflow', '🧪', 'ae-test', 'Seeded test workflow for CLIENT stage', 'active', ARRAY['CLIENT'], 'seed')
		ON CONFLICT (workspace_id, slug) DO UPDATE SET status = 'active', updated_at = NOW()
		RETURNING id`,
		wsID).Scan(&wfID)
	if err != nil {
		log.Printf("upsert workflow: %v", err)
		return ""
	}
	return wfID
}

func seedWorkflowNodes(db *sql.DB, wfID string) {
	// Replace all nodes for a clean slate.
	if _, err := db.Exec(`DELETE FROM workflow_nodes WHERE workflow_id = $1`, wfID); err != nil {
		log.Printf("delete workflow_nodes: %v", err)
		return
	}

	nodes, _ := workflowCanvas()
	for _, n := range nodes {
		dataJSON, _ := json.Marshal(n.data)
		_, err := db.Exec(`
			INSERT INTO workflow_nodes
			  (workflow_id, node_id, node_type, position_x, position_y, width, height, data)
			VALUES ($1, $2, 'workflow', $3, $4, 240, 80, $5)
			ON CONFLICT (workflow_id, node_id) DO NOTHING`,
			wfID, n.nodeID, n.x, n.y, string(dataJSON))
		if err != nil {
			log.Printf("insert node %s: %v", n.nodeID, err)
		}
	}
}

func seedWorkflowEdges(db *sql.DB, wfID string) {
	if _, err := db.Exec(`DELETE FROM workflow_edges WHERE workflow_id = $1`, wfID); err != nil {
		log.Printf("delete workflow_edges: %v", err)
		return
	}

	_, edges := workflowCanvas()
	for i, e := range edges {
		edgeID := fmt.Sprintf("edge-%d", i+1)
		_, err := db.Exec(`
			INSERT INTO workflow_edges
			  (workflow_id, edge_id, source_node_id, target_node_id, animated)
			VALUES ($1, $2, $3, $4, TRUE)
			ON CONFLICT (workflow_id, edge_id) DO NOTHING`,
			wfID, edgeID, e[0], e[1])
		if err != nil {
			log.Printf("insert edge %s: %v", edgeID, err)
		}
	}
}

func seedAutomationRules(db *sql.DB, wsID string) int {
	count := 0
	for _, r := range automationRules {
		_, err := db.Exec(`
			INSERT INTO automation_rules
			  (workspace_id, rule_code, trigger_id, role, phase, phase_label,
			   priority, timing, condition, stop_if, sent_flag, channel, status)
			VALUES ($1, $2, $3, 'ae', $4, $4, 'P1', '-', $5, '-', $6, 'whatsapp', 'active')
			ON CONFLICT (workspace_id, rule_code) DO UPDATE
			  SET condition = EXCLUDED.condition,
			      sent_flag = EXCLUDED.sent_flag,
			      status    = 'active',
			      updated_at = NOW()`,
			wsID, r.ruleCode, r.ruleCode, r.phase, r.condition, r.sentFlag)
		if err != nil {
			log.Printf("  ⚠️  rule %s: %v", r.ruleCode, err)
		} else {
			count++
		}
	}
	return count
}

func seedWorkflowClients(db *sql.DB, wsID, titled, prefix string) int {
	const q = `
INSERT INTO clients (
  company_id, company_name, pic_name, pic_wa, pic_email, pic_role,
  owner_name, owner_wa, owner_telegram_id,
  payment_terms,
  contract_start, contract_end, contract_months, activation_date,
  payment_status, blacklisted,
  stage, risk_flag_text, days_to_expiry, custom_fields, workspace_id
) VALUES (
  $1, $2, $3, $4, $5, 'HR Manager',
  $6, $7, $8,
  'Net 30',
  CURRENT_DATE + $9::int, CURRENT_DATE + $10::int, 12, CURRENT_DATE + $9::int + 14,
  $11, FALSE,
  'CLIENT', $12, $10, $13::jsonb, $14
)
ON CONFLICT (company_id) DO NOTHING`

	count := 0
	for i, s := range workflowScenarios {
		companyID := fmt.Sprintf("%s%02d", prefix, s.n)
		segment := map[int]string{1: "High", 2: "Mid", 3: "Mid", 4: "Low"}[s.n]
		_, err := db.Exec(q,
			companyID,
			fmt.Sprintf("Seed WF %s %02d", titled, s.n),
			fmt.Sprintf("PIC WF %s %d", titled, s.n),
			fmt.Sprintf("+6287%09d", (i+1)*100+s.n),
			fmt.Sprintf("wf%d@seed-%s.test", s.n, strings.ToLower(titled)),
			fmt.Sprintf("Owner WF %s %d", titled, s.n),
			fmt.Sprintf("+6286%09d", (i+1)*100+s.n),
			fmt.Sprintf("8%08d", (i+1)*100+s.n),
			s.cStartOff, s.cEndOff,
			s.payStatus,
			s.riskFlag,
			customFieldsJSON(s.nps, s.usage, s.planType, segment),
			wsID,
		)
		if err != nil {
			log.Printf("  ⚠️  insert %s: %v", companyID, err)
		} else {
			fmt.Printf("   %-20s  %-22s  DTE=%-5d  risk=%-4s  → expects: %s\n",
				companyID, s.label, s.cEndOff, s.riskFlag, s.triggerRule)
			count++
		}
	}
	return count
}

// customFieldsJSON encodes per-client custom fields as a JSON string for the
// custom_fields JSONB column. Fields nps_score, usage_score, plan_type, and
// segment were dropped as native columns in migrations 20260427000200/000300.
func customFieldsJSON(nps, usage int, planType, segment string) string {
	return fmt.Sprintf(`{"nps_score":%d,"usage_score":%d,"plan_type":%q,"segment":%q}`,
		nps, usage, planType, segment)
}

// seedCustomFieldDefs ensures the workspace has the required custom_field_definitions
// rows before inserting clients whose custom_fields reference them.
func seedCustomFieldDefs(db *sql.DB, wsID string) {
	defs := []struct{ key, label, fieldType string }{
		{"nps_score", "NPS Score", "number"},
		{"usage_score", "Usage Score", "number"},
		{"plan_type", "Plan Type", "text"},
		{"hc_size", "HC Size", "number"},
	}
	for _, d := range defs {
		if _, err := db.Exec(`
			INSERT INTO custom_field_definitions (workspace_id, field_key, field_label, field_type)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (workspace_id, field_key) DO NOTHING`,
			wsID, d.key, d.label, d.fieldType); err != nil {
			log.Printf("  ⚠️  custom_field_def %s: %v", d.key, err)
		}
	}
}

// ── Trigger + poll helpers ────────────────────────────────────────────────────

func extractJobID(body []byte, wsID string) string {
	var resp struct {
		Data []struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspace_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}
	for _, j := range resp.Data {
		if j.WorkspaceID == wsID {
			return j.ID
		}
	}
	return ""
}

func pollJob(db *sql.DB, wsID, jobID string, since time.Time) string {
	fmt.Print("⏳ Waiting for job")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		fmt.Print(".")

		var id, status string
		var processed, success, failed int
		var err error

		if jobID != "" {
			err = db.QueryRow(
				`SELECT id, status, processed, success, failed FROM background_jobs WHERE id = $1`,
				jobID).Scan(&id, &status, &processed, &success, &failed)
		} else {
			err = db.QueryRow(`
				SELECT id, status, processed, success, failed FROM background_jobs
				WHERE workspace_id = $1 AND job_type = 'cron' AND created_at > $2
				ORDER BY created_at DESC LIMIT 1`,
				wsID, since).Scan(&id, &status, &processed, &success, &failed)
		}
		if err != nil {
			continue
		}
		jobID = id
		if status == "done" || status == "failed" {
			fmt.Printf(" %s\n", status)
			fmt.Printf("   processed=%d  success=%d  failed=%d\n", processed, success, failed)
			return jobID
		}
	}
	fmt.Println(" timeout")
	return jobID
}

// ── Results ───────────────────────────────────────────────────────────────────

func printResults(db *sql.DB, wsID, jobID, legacyPrefix, wfPrefix string) {
	printActionLog(db, wsID, legacyPrefix, "LEGACY runner  (action_log)")
	fmt.Println()
	printActionLogs(db, wsID, wfPrefix, "WORKFLOW engine (action_logs)")
	fmt.Println()

	if jobID != "" {
		fmt.Printf("📊 Job detail   : SELECT * FROM background_jobs WHERE id = '%s';\n", jobID)
	}
	fmt.Printf("📊 Legacy log   : SELECT * FROM action_log WHERE company_id LIKE '%s%%' ORDER BY triggered_at DESC;\n", legacyPrefix)
	fmt.Printf("📊 Workflow log : SELECT * FROM action_logs WHERE master_data_id IN (SELECT master_id FROM clients WHERE company_id LIKE '%s%%') ORDER BY timestamp DESC;\n", wfPrefix)
}

func printActionLog(db *sql.DB, wsID, prefix, header string) {
	fmt.Printf("── %s ──\n", header)
	fmt.Printf("  %-22s  %-22s  %-30s  %-10s\n", "COMPANY_ID", "TRIGGER", "TEMPLATE", "STATUS")

	rows, err := db.Query(`
		SELECT al.company_id, al.trigger_type, COALESCE(al.template_id,'—'), COALESCE(al.status,'—')
		FROM action_log al
		JOIN clients c ON c.company_id = al.company_id
		WHERE c.workspace_id = $1 AND c.company_id LIKE $2
		  AND al.triggered_at > NOW() - INTERVAL '5 minutes'
		ORDER BY al.company_id, al.triggered_at DESC`,
		wsID, prefix+"%")
	if err != nil {
		fmt.Printf("  (query error: %v)\n", err)
		return
	}
	defer rows.Close()
	printRows(rows)
}

func printActionLogs(db *sql.DB, wsID, prefix, header string) {
	fmt.Printf("── %s ──\n", header)
	fmt.Printf("  %-22s  %-22s  %-10s\n", "MASTER_DATA_ID", "TRIGGER_ID", "STATUS")

	rows, err := db.Query(`
		SELECT al.master_data_id::text, COALESCE(al.trigger_id,'—'), al.status
		FROM action_logs al
		JOIN clients c ON c.master_id::text = al.master_data_id::text
		WHERE al.workspace_id = $1 AND c.company_id LIKE $2
		  AND al.timestamp > NOW() - INTERVAL '5 minutes'
		ORDER BY c.company_id, al.timestamp DESC`,
		wsID, prefix+"%")
	if err != nil {
		fmt.Printf("  (query error: %v)\n", err)
		return
	}
	defer rows.Close()
	printRows(rows)
}

func printRows(rows *sql.Rows) {
	cols, _ := rows.Columns()
	found := 0
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		parts := make([]string, len(cols))
		for i, v := range vals {
			parts[i] = fmt.Sprintf("%-22v", v)
		}
		fmt.Printf("  %s\n", strings.Join(parts, "  "))
		found++
	}
	if found == 0 {
		fmt.Println("  (no entries — see gate-check reasons below)")
		fmt.Println("  • sentToday: message already sent to this client today")
		fmt.Println("  • BotActive=false or Blacklisted=true")
		fmt.Println("  • Condition not matched for today's dates")
		fmt.Println("  • USE_WORKFLOW_ENGINE=false (workflow path skipped)")
	}
}
