package migrations

import (
	"strings"
	"testing"
)

func TestLoadIncludesBaselineMigration(t *testing.T) {
	migrations, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("Load() returned no migrations")
	}
	if migrations[0].Version != 1 || migrations[0].Name != "initial_schema" {
		t.Fatalf("first migration = %#v, want 0001_initial_schema", migrations[0])
	}
}

func TestLoadIncludesStudentGeneratedExerciseMigration(t *testing.T) {
	migrations, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	var migrationSQL string
	for _, migration := range migrations {
		if migration.Version == 3 && migration.Name == "student_generated_exercises" {
			migrationSQL = migration.SQL
			break
		}
	}
	if migrationSQL == "" {
		t.Fatal("Load() missing 0003_student_generated_exercises")
	}

	for _, want := range []string{
		"ADD COLUMN generated_by_student_id",
		"ALTER COLUMN owner_teacher_id DROP NOT NULL",
		"contents_generated_by_student_id_fkey",
		"ck_contents_exactly_one_owner",
		"(owner_teacher_id IS NOT NULL) <> (generated_by_student_id IS NOT NULL)",
		"ck_contents_student_generated_problem",
		"generated_by_student_id IS NULL OR type = 'PROBLEM'::public.contenttype",
		"WHERE generated_by_student_id IS NOT NULL AND deleted_at IS NULL",
	} {
		if !strings.Contains(migrationSQL, want) {
			t.Fatalf("migration SQL missing %q", want)
		}
	}
}

func TestLoadIncludesStudentAIRiskControlMigration(t *testing.T) {
	migrations, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	var migrationSQL string
	for _, migration := range migrations {
		if migration.Version == 4 && migration.Name == "student_ai_risk_control" {
			migrationSQL = migration.SQL
			break
		}
	}
	if migrationSQL == "" {
		t.Fatal("Load() missing 0004_student_ai_risk_control")
	}
	for _, want := range []string{
		"student_ai_daily_reply_limit",
		"student_ai_max_concurrency",
		"student_ai_access_controls",
		"student_ai_reply_usage",
		"student_ai_risk_events",
		"uq_student_ai_reply_usage_message",
		"ON DELETE SET NULL",
	} {
		if !strings.Contains(migrationSQL, want) {
			t.Fatalf("migration SQL missing %q", want)
		}
	}
}
