package migration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LegacyMistakeBookConfirmation is the confirmation value required before
// changing the migration ledger. Keep this deliberately specific and stable.
const LegacyMistakeBookConfirmation = "RECONCILE_0011_MISTAKE_BOOK_INTEGRITY"

const (
	legacyMistakeBookIntegrity      = "mistake_book_integrity"
	legacyMistakeBookReconciliation = "mistake_book_schema_reconciliation"
	legacyAttemptSnapshots          = "attempt_question_snapshots"
	uncategorizedKnowledgeNodeID    = "00000000-0000-0000-0000-000000000001"
)

// ReconcileOptions controls the legacy ledger reconciliation operation.
// Apply is intentionally false by default; the check path does not mutate the
// database and is suitable for a preflight run.
type ReconcileOptions struct {
	Apply        bool
	Confirmation string
	Environment  string
}

// ReconcileResult describes a successful preflight or reconciliation.
type ReconcileResult struct {
	Applied     bool
	LegacyRows  int
	DeletedRows int64
}

// ReconcileLegacyMistakeBookLedger verifies the exact local database shape
// produced by the unpublished mistake-book migrations. It accepts the two
// known local ledger shapes: the database stopped after legacy version 11, or
// it also ran the later unpublished versions 12 and 13. In apply mode it only
// changes migration metadata: business tables and rows are never modified.
// The ordinary migration runner intentionally does not call this function.
func ReconcileLegacyMistakeBookLedger(
	ctx context.Context,
	pool *pgxpool.Pool,
	canonical []Migration,
	opts ReconcileOptions,
) (ReconcileResult, error) {
	if pool == nil {
		return ReconcileResult{}, errors.New("postgres pool is nil")
	}
	if err := validateReconcileOptions(opts); err != nil {
		return ReconcileResult{}, err
	}
	if err := validateCanonicalChain(canonical); err != nil {
		return ReconcileResult{}, err
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("begin reconciliation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "LOCK TABLE public.go_schema_migrations IN ACCESS EXCLUSIVE MODE"); err != nil {
		return ReconcileResult{}, fmt.Errorf("lock public.go_schema_migrations; this command requires an existing legacy ledger: %w", err)
	}

	rows, err := readLedger(ctx, tx)
	if err != nil {
		return ReconcileResult{}, err
	}
	if err := validateLegacyLedger(rows, canonical); err != nil {
		return ReconcileResult{}, err
	}
	if err := validateMergedMistakeBookSchema(ctx, tx); err != nil {
		return ReconcileResult{}, err
	}

	result := ReconcileResult{LegacyRows: len(rows)}
	if !opts.Apply {
		return result, nil
	}

	legacyVersions := legacyLedgerVersions(rows)
	deleteTag, err := tx.Exec(ctx, `
		DELETE FROM public.go_schema_migrations
		WHERE (version, name) IN (
			(11, $1),
			(12, $2),
			(13, $3)
		)
	`, legacyMistakeBookIntegrity, legacyMistakeBookReconciliation, legacyAttemptSnapshots)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("remove legacy migration metadata: %w", err)
	}
	if deleteTag.RowsAffected() != int64(len(legacyVersions)) {
		return ReconcileResult{}, fmt.Errorf("remove legacy migration metadata: expected %d rows, deleted %d", len(legacyVersions), deleteTag.RowsAffected())
	}
	remaining, err := readLedger(ctx, tx)
	if err != nil {
		return ReconcileResult{}, err
	}
	if err := validateReconciledLedger(remaining, canonical); err != nil {
		return ReconcileResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReconcileResult{}, fmt.Errorf("commit migration ledger reconciliation: %w", err)
	}
	result.Applied = true
	result.DeletedRows = deleteTag.RowsAffected()
	return result, nil
}

type ledgerRow struct {
	version   int64
	name      string
	appliedAt time.Time
}

func validateReconcileOptions(opts ReconcileOptions) error {
	switch strings.ToLower(strings.TrimSpace(opts.Environment)) {
	case "development", "dev", "local", "test":
		// This command is only for disposable or explicitly local development
		// environments. Unknown names fail closed because they may identify a
		// shared deployment.
	default:
		return fmt.Errorf("ledger reconciliation is only allowed in an explicit local environment, got %q", opts.Environment)
	}
	if !opts.Apply {
		if strings.TrimSpace(opts.Confirmation) != "" {
			return errors.New("--confirm requires --apply")
		}
		return nil
	}
	if opts.Confirmation != LegacyMistakeBookConfirmation {
		return fmt.Errorf("invalid confirmation; pass --confirm %s", LegacyMistakeBookConfirmation)
	}
	return nil
}

func validateCanonicalChain(migrations []Migration) error {
	if len(migrations) == 0 {
		return errors.New("canonical migration list is empty")
	}
	seen := make(map[int64]string, len(migrations))
	for _, item := range migrations {
		if item.Version <= 0 {
			return fmt.Errorf("canonical migration version must be positive, got %d", item.Version)
		}
		if _, exists := seen[item.Version]; exists {
			return fmt.Errorf("canonical migrations contain duplicate version %d", item.Version)
		}
		seen[item.Version] = item.Name
	}
	if len(seen) < 11 {
		return fmt.Errorf("ledger reconciliation requires the canonical prefix 1-11, found %d migrations", len(seen))
	}
	for version := int64(1); version <= int64(len(seen)); version++ {
		if _, ok := seen[version]; !ok {
			return fmt.Errorf("canonical migration %d is missing; versions must be contiguous from 1", version)
		}
	}
	if seen[11] != "forum_center" {
		return fmt.Errorf("canonical migration 11 must be forum_center, got %q", seen[11])
	}
	if seen[10] != "mistake_review_tasks" {
		return fmt.Errorf("canonical migration 10 must be mistake_review_tasks, got %q", seen[10])
	}
	return nil
}

func canonicalName(migrations []Migration, version int64) string {
	for _, item := range migrations {
		if item.Version == version {
			return item.Name
		}
	}
	return ""
}

func readLedger(ctx context.Context, tx pgx.Tx) ([]ledgerRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT version, name, applied_at
		FROM public.go_schema_migrations
		ORDER BY version
	`)
	if err != nil {
		return nil, fmt.Errorf("read migration ledger: %w", err)
	}
	defer rows.Close()
	result := make([]ledgerRow, 0)
	for rows.Next() {
		var item ledgerRow
		if err := rows.Scan(&item.version, &item.name, &item.appliedAt); err != nil {
			return nil, fmt.Errorf("scan migration ledger: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read migration ledger rows: %w", err)
	}
	return result, nil
}

func validateLegacyLedger(rows []ledgerRow, canonical []Migration) error {
	if len(rows) != 11 && len(rows) != 13 {
		return fmt.Errorf("legacy ledger must contain exactly 11 or 13 rows, found %d", len(rows))
	}
	for index, item := range rows {
		expectedVersion := int64(index + 1)
		if item.version != expectedVersion {
			return fmt.Errorf("legacy ledger must contain contiguous versions 1-%d; row %d has version %d", len(rows), index, item.version)
		}
		if item.appliedAt.IsZero() {
			return fmt.Errorf("legacy ledger version %d has an empty applied_at", item.version)
		}
		expectedName := canonicalName(canonical, item.version)
		switch item.version {
		case 11:
			expectedName = legacyMistakeBookIntegrity
		case 12:
			if len(rows) != 13 {
				return fmt.Errorf("legacy ledger version 12 is only valid with version 13")
			}
			expectedName = legacyMistakeBookReconciliation
		case 13:
			if len(rows) != 13 {
				return fmt.Errorf("legacy ledger version 13 requires the complete 11-13 legacy suffix")
			}
			expectedName = legacyAttemptSnapshots
		}
		if item.name != expectedName {
			return fmt.Errorf("legacy ledger version %d must be %q, found %q", item.version, expectedName, item.name)
		}
	}
	return nil
}

func legacyLedgerVersions(rows []ledgerRow) []int64 {
	versions := []int64{11}
	if len(rows) == 13 {
		versions = append(versions, 12, 13)
	}
	return versions
}

func validateReconciledLedger(rows []ledgerRow, canonical []Migration) error {
	if len(rows) != 10 {
		return fmt.Errorf("reconciled ledger must contain exactly 10 rows, found %d", len(rows))
	}
	for index, item := range rows {
		expectedVersion := int64(index + 1)
		if item.version != expectedVersion {
			return fmt.Errorf("reconciled ledger must contain contiguous versions 1-10; row %d has version %d", index, item.version)
		}
		expectedName := canonicalName(canonical, item.version)
		if item.name != expectedName {
			return fmt.Errorf("reconciled ledger version %d must be %q, found %q", item.version, expectedName, item.name)
		}
	}
	return nil
}

func validateMergedMistakeBookSchema(ctx context.Context, tx pgx.Tx) error {
	// A partial forum migration would make the subsequent canonical migration
	// unsafe, so reject any forum relation rather than attempting cleanup.
	for _, table := range []string{
		"forum_boards", "forum_posts", "forum_replies", "forum_post_likes",
		"forum_post_favorites", "forum_reports", "forum_notifications",
	} {
		exists, err := relationExists(ctx, tx, table)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("legacy database already contains public.%s; refusing to reconcile a partial forum migration", table)
		}
	}

	for table, columns := range map[string][]string{
		"mistake_review_tasks": {
			"id", "student_id", "content_id", "question_title", "question_body",
			"question_difficulty", "question_concept_ids", "question_meta",
			"question_generated_by_student_id", "source_attempt_id", "daily_assignment_id",
			"status", "stage", "review_count", "successful_review_count", "error_count",
			"due_at", "last_review_attempt_id", "last_outcome", "last_reviewed_at",
			"mastered_at", "archived_at", "revision", "created_at", "updated_at",
		},
		"mistake_record_archives": {"attempt_id", "student_id", "archived_at"},
	} {
		if err := requireColumns(ctx, tx, table, columns); err != nil {
			return err
		}
	}

	if err := requireColumns(ctx, tx, "content_attempts", []string{
		"id", "student_id", "content_id", "submitted_at", "review_task_id",
		"review_submission_key", "review_submission_response", "review_question_title",
		"review_question_body", "review_question_concept_ids", "review_question_difficulty",
		"review_question_meta", "review_question_generated_by_student_id", "mastery_weight",
		"review_submission_digest", "mistake_redo_original_attempt_id", "mistake_redo_submission_id",
		"mistake_redo_submission_digest", "mistake_redo_submission_response", "regular_submission_id",
		"regular_submission_digest", "regular_submission_response",
	}); err != nil {
		return err
	}

	for _, constraint := range []string{
		"mistake_review_tasks_pkey", "uq_mistake_review_task_student_content",
		"ck_mistake_review_tasks_status", "ck_mistake_review_tasks_stage",
		"ck_mistake_review_tasks_counts", "ck_mistake_review_tasks_revision",
		"ck_mistake_review_tasks_state_dates", "mistake_review_tasks_student_id_fkey",
		"mistake_review_tasks_content_id_fkey", "mistake_review_tasks_source_attempt_id_fkey",
		"mistake_review_tasks_daily_assignment_id_fkey", "mistake_review_tasks_last_review_attempt_id_fkey",
		"mistake_record_archives_pkey", "uq_content_attempts_id_student",
		"mistake_record_archives_attempt_student_fkey", "mistake_record_archives_student_id_fkey",
		"ck_content_attempts_mastery_weight", "content_attempts_review_task_id_fkey",
		"ck_contents_concept_ids_array", "ck_daily_question_assignment_concept_ids_array",
		"ck_daily_question_selection_concept_ids_array", "ck_mistake_review_tasks_concept_ids_array",
		"ck_content_attempts_review_concept_ids_array", "ck_contents_problem_concepts_nonempty",
		"ck_daily_question_assignment_concepts_nonempty", "ck_daily_question_selection_concepts_nonempty",
		"ck_mistake_review_tasks_concepts_nonempty", "ck_content_attempts_review_concepts_nonempty",
		"ck_content_attempts_review_submission_digest_pair", "ck_content_attempts_review_submission_digest",
		"content_attempts_mistake_redo_original_attempt_id_fkey", "ck_content_attempts_mistake_redo_submission",
		"ck_content_attempts_mistake_redo_digest", "ck_content_attempts_regular_submission",
		"ck_content_attempts_regular_submission_digest", "ck_mistake_review_tasks_count_order",
		"ck_mistake_review_tasks_stage_progress", "ck_student_profiles_mastery_vector_object",
		"student_concept_dkt_states_concept_id_fkey", "ck_content_attempts_review_question_snapshot",
	} {
		if err := requireConstraint(ctx, tx, constraint); err != nil {
			return err
		}
	}

	for _, index := range []string{
		"ix_mistake_review_tasks_due", "ix_mistake_review_tasks_mastered",
		"ix_mistake_record_archives_student", "ix_content_attempts_review_task",
		"uq_content_attempts_review_submission", "uq_content_attempts_mistake_redo_submission",
		"ix_content_attempts_mistake_redo_original", "uq_content_attempts_regular_submission",
		"ix_content_attempts_mistake_aggregate",
	} {
		if err := requireIndex(ctx, tx, index); err != nil {
			return err
		}
	}

	if err := validateMergedData(ctx, tx); err != nil {
		return err
	}
	return nil
}

func relationExists(ctx context.Context, tx pgx.Tx, table string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists); err != nil {
		return false, fmt.Errorf("check relation public.%s: %w", table, err)
	}
	return exists, nil
}

func requireColumns(ctx context.Context, tx pgx.Tx, table string, columns []string) error {
	rows, err := tx.Query(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1 AND column_name = ANY($2::text[])
	`, table, columns)
	if err != nil {
		return fmt.Errorf("inspect columns for public.%s: %w", table, err)
	}
	defer rows.Close()
	found := make(map[string]bool, len(columns))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan columns for public.%s: %w", table, err)
		}
		found[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read columns for public.%s: %w", table, err)
	}
	missing := make([]string, 0)
	for _, column := range columns {
		if !found[column] {
			missing = append(missing, column)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("public.%s is missing expected columns: %s", table, strings.Join(missing, ", "))
	}
	return nil
}

func requireConstraint(ctx context.Context, tx pgx.Tx, name string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint constraint_row
			JOIN pg_namespace namespace_row ON namespace_row.oid = constraint_row.connamespace
			WHERE namespace_row.nspname = 'public'
			  AND constraint_row.conname = $1
			  AND constraint_row.convalidated
		)
	`, name).Scan(&exists); err != nil {
		return fmt.Errorf("inspect constraint %s: %w", name, err)
	}
	if !exists {
		return fmt.Errorf("expected constraint %s is missing", name)
	}
	return nil
}

func requireIndex(ctx context.Context, tx pgx.Tx, name string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_class index_row
			JOIN pg_namespace namespace_row ON namespace_row.oid = index_row.relnamespace
			JOIN pg_index index_state ON index_state.indexrelid = index_row.oid
			WHERE namespace_row.nspname = 'public'
			  AND index_row.relkind IN ('i', 'I')
			  AND index_row.relname = $1
			  AND index_state.indisvalid
			  AND index_state.indisready
		)
	`, name).Scan(&exists); err != nil {
		return fmt.Errorf("inspect index %s: %w", name, err)
	}
	if !exists {
		return fmt.Errorf("expected index %s is missing", name)
	}
	return nil
}

func validateMergedData(ctx context.Context, tx pgx.Tx) error {
	checks := []struct {
		name     string
		query    string
		args     []any
		expected int64
	}{
		{
			name:  "submitted attempt snapshots",
			query: `SELECT count(*) FROM public.content_attempts WHERE submitted_at IS NOT NULL AND (review_question_title IS NULL OR review_question_body IS NULL OR review_question_concept_ids IS NULL OR review_question_difficulty IS NULL OR review_question_meta IS NULL)`,
		},
		{
			name:  "content concept arrays",
			query: `SELECT count(*) FROM public.contents WHERE json_typeof(concept_ids) IS DISTINCT FROM 'array'`,
		},
		{
			name:  "nonempty problem concepts",
			query: `SELECT count(*) FROM public.contents WHERE type = 'PROBLEM'::public.contenttype AND json_array_length(concept_ids) = 0`,
		},
		{
			name:  "daily assignment concept arrays",
			query: `SELECT count(*) FROM public.daily_question_assignments WHERE question_concept_ids IS NOT NULL AND NOT (CASE WHEN json_typeof(question_concept_ids) = 'array' THEN json_array_length(question_concept_ids) > 0 ELSE false END)`,
		},
		{
			name:  "daily selection concept arrays",
			query: `SELECT count(*) FROM public.daily_question_class_selections WHERE question_concept_ids IS NOT NULL AND NOT (CASE WHEN json_typeof(question_concept_ids) = 'array' THEN json_array_length(question_concept_ids) > 0 ELSE false END)`,
		},
		{
			name:  "review task concept arrays",
			query: `SELECT count(*) FROM public.mistake_review_tasks WHERE NOT (CASE WHEN json_typeof(question_concept_ids) = 'array' THEN json_array_length(question_concept_ids) > 0 ELSE false END)`,
		},
		{
			name:  "review attempt concept arrays",
			query: `SELECT count(*) FROM public.content_attempts WHERE review_question_concept_ids IS NOT NULL AND NOT (CASE WHEN json_typeof(review_question_concept_ids) = 'array' THEN json_array_length(review_question_concept_ids) > 0 ELSE false END)`,
		},
		{
			name:  "orphan dkt states",
			query: `SELECT count(*) FROM public.student_concept_dkt_states state_row WHERE NOT EXISTS (SELECT 1 FROM public.knowledge_nodes node WHERE node.id = state_row.concept_id)`,
		},
		{
			name:     "uncategorized knowledge node",
			query:    `SELECT count(*) FROM public.knowledge_nodes WHERE id = $1 AND name = '未分类'`,
			args:     []any{uncategorizedKnowledgeNodeID},
			expected: 1,
		},
		{
			name:  "stale migration helper functions",
			query: `SELECT count(*) FROM pg_proc procedure_row JOIN pg_namespace namespace_row ON namespace_row.oid = procedure_row.pronamespace WHERE namespace_row.nspname = 'public' AND (procedure_row.proname LIKE 'msp_0011_%' OR procedure_row.proname LIKE 'msp_0012_%')`,
		},
	}
	for _, check := range checks {
		var count int64
		if err := tx.QueryRow(ctx, check.query, check.args...).Scan(&count); err != nil {
			return fmt.Errorf("validate %s: %w", check.name, err)
		}
		if count != check.expected {
			return fmt.Errorf("validate %s: expected %d rows, found %d", check.name, check.expected, count)
		}
	}

	// Once the collection shape is known, ensure every referenced concept ID is
	// still a real node. This query is intentionally read-only and runs before
	// any ledger mutation.
	conceptQueries := []struct {
		name  string
		query string
	}{
		{"content concept references", `SELECT count(*) FROM public.contents content, LATERAL json_array_elements_text(content.concept_ids) concept WHERE NOT EXISTS (SELECT 1 FROM public.knowledge_nodes node WHERE node.id = btrim(concept.value))`},
		{"daily assignment concept references", `SELECT count(*) FROM public.daily_question_assignments assignment, LATERAL json_array_elements_text(assignment.question_concept_ids) concept WHERE assignment.question_concept_ids IS NOT NULL AND NOT EXISTS (SELECT 1 FROM public.knowledge_nodes node WHERE node.id = btrim(concept.value))`},
		{"daily selection concept references", `SELECT count(*) FROM public.daily_question_class_selections selection, LATERAL json_array_elements_text(selection.question_concept_ids) concept WHERE selection.question_concept_ids IS NOT NULL AND NOT EXISTS (SELECT 1 FROM public.knowledge_nodes node WHERE node.id = btrim(concept.value))`},
		{"review task concept references", `SELECT count(*) FROM public.mistake_review_tasks task, LATERAL json_array_elements_text(task.question_concept_ids) concept WHERE NOT EXISTS (SELECT 1 FROM public.knowledge_nodes node WHERE node.id = btrim(concept.value))`},
		{"review attempt concept references", `SELECT count(*) FROM public.content_attempts attempt, LATERAL json_array_elements_text(attempt.review_question_concept_ids) concept WHERE attempt.review_question_concept_ids IS NOT NULL AND NOT EXISTS (SELECT 1 FROM public.knowledge_nodes node WHERE node.id = btrim(concept.value))`},
		{"profile mastery concept references", `SELECT count(*) FROM public.student_profiles profile, LATERAL json_each(profile.mastery_vector) mastery WHERE NOT EXISTS (SELECT 1 FROM public.knowledge_nodes node WHERE node.id = mastery.key)`},
	}
	for _, check := range conceptQueries {
		var count int64
		if err := tx.QueryRow(ctx, check.query).Scan(&count); err != nil {
			return fmt.Errorf("validate %s: %w", check.name, err)
		}
		if count != 0 {
			return fmt.Errorf("validate %s: found %d orphan references", check.name, count)
		}
	}
	return nil
}
