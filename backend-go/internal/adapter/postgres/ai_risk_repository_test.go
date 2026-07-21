package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	airiskapp "mathstudy/backend-go/internal/application/airisk"
)

type scanRowFunc func(...any) error

func (f scanRowFunc) Scan(dest ...any) error { return f(dest...) }

type staticAIRiskRows struct {
	pgx.Rows
	scans []scanRowFunc
	index int
	err   error
}

func (r *staticAIRiskRows) Close() {}

func (r *staticAIRiskRows) Err() error { return r.err }

func (r *staticAIRiskRows) Next() bool {
	if r.index >= len(r.scans) {
		return false
	}
	r.index++
	return true
}

func (r *staticAIRiskRows) Scan(dest ...any) error {
	return r.scans[r.index-1](dest...)
}

type recordingAIRiskQuerier struct {
	execSQL      []string
	execArgs     [][]any
	querySQL     string
	queryArgs    []any
	queryRowSQL  string
	queryRowArgs []any
	rows         pgx.Rows
	row          pgx.Row
	execErr      error
	queryErr     error
}

func (q *recordingAIRiskQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execSQL = append(q.execSQL, sql)
	q.execArgs = append(q.execArgs, append([]any(nil), args...))
	return pgconn.CommandTag{}, q.execErr
}

func (q *recordingAIRiskQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.querySQL = sql
	q.queryArgs = append([]any(nil), args...)
	return q.rows, q.queryErr
}

func (q *recordingAIRiskQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.queryRowSQL = sql
	q.queryRowArgs = append([]any(nil), args...)
	return q.row
}

type recordingAIRiskTx struct {
	pgx.Tx
	recordingAIRiskQuerier
	commits   int
	rollbacks int
}

func (tx *recordingAIRiskTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tx.recordingAIRiskQuerier.Exec(ctx, sql, args...)
}

func (tx *recordingAIRiskTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return tx.recordingAIRiskQuerier.Query(ctx, sql, args...)
}

func (tx *recordingAIRiskTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.recordingAIRiskQuerier.QueryRow(ctx, sql, args...)
}

func (tx *recordingAIRiskTx) Commit(context.Context) error {
	tx.commits++
	return nil
}

func (tx *recordingAIRiskTx) Rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

type recordingAIRiskBeginner struct {
	recordingAIRiskQuerier
	tx     *recordingAIRiskTx
	begins int
}

func (b *recordingAIRiskBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	b.begins++
	return b.tx, nil
}

func TestNewAIRiskRepositoryRejectsNilQuerier(t *testing.T) {
	if _, err := NewAIRiskRepository(nil); err == nil {
		t.Fatal("NewAIRiskRepository(nil) error = nil")
	}
}

func TestAIRiskRepositoryLoadsSettingsAndStudentAccess(t *testing.T) {
	settingsRows := &staticAIRiskRows{scans: []scanRowFunc{
		func(dest ...any) error {
			*dest[0].(*string) = airiskapp.DailyReplyLimitKey
			*dest[1].(*string) = "50"
			return nil
		},
		func(dest ...any) error {
			*dest[0].(*string) = airiskapp.MaxConcurrencyKey
			*dest[1].(*string) = "2"
			return nil
		},
	}}
	querier := &recordingAIRiskQuerier{rows: settingsRows}
	repo, err := NewAIRiskRepository(querier)
	if err != nil {
		t.Fatalf("NewAIRiskRepository() error = %v", err)
	}

	settings, err := repo.GetSettings(context.Background(), []string{airiskapp.DailyReplyLimitKey, airiskapp.MaxConcurrencyKey})
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if settings[airiskapp.DailyReplyLimitKey] != "50" || settings[airiskapp.MaxConcurrencyKey] != "2" {
		t.Fatalf("settings = %#v", settings)
	}
	if !strings.Contains(querier.querySQL, "WHERE key = ANY($1)") || len(querier.queryArgs) != 1 {
		t.Fatalf("settings query = %s args=%#v", querier.querySQL, querier.queryArgs)
	}

	blockedAt := time.Date(2026, 7, 21, 3, 0, 0, 0, time.UTC)
	querier.row = scanRowFunc(func(dest ...any) error {
		*dest[0].(*string) = "student-1"
		*dest[1].(*string) = "alice"
		*dest[2].(*string) = "STUDENT"
		*dest[3].(*bool) = true
		*dest[4].(*pgtype.Text) = pgtype.Text{String: "违规", Valid: true}
		*dest[5].(*pgtype.Timestamp) = pgtype.Timestamp{Time: blockedAt, Valid: true}
		return nil
	})
	access, found, err := repo.GetStudentAccess(context.Background(), "student-1")
	if err != nil || !found {
		t.Fatalf("GetStudentAccess() = %#v, %v, %v", access, found, err)
	}
	if !access.IsStudent || !access.IsBlocked || access.BlockedReason != "违规" || access.BlockedAt == nil || !access.BlockedAt.Equal(blockedAt) {
		t.Fatalf("access = %#v", access)
	}
}

func TestAIRiskRepositoryReadsUsageAndOverview(t *testing.T) {
	querier := &recordingAIRiskQuerier{}
	repo, err := NewAIRiskRepository(querier)
	if err != nil {
		t.Fatalf("NewAIRiskRepository() error = %v", err)
	}
	querier.row = scanRowFunc(func(dest ...any) error {
		*dest[0].(*int) = 7
		return nil
	})
	count, err := repo.CountReplies(context.Background(), "student-1", "2026-07-21")
	if err != nil || count != 7 {
		t.Fatalf("CountReplies() = %d, %v", count, err)
	}
	if !strings.Contains(querier.queryRowSQL, "student_ai_reply_usage") || len(querier.queryRowArgs) != 2 {
		t.Fatalf("count query = %s args=%#v", querier.queryRowSQL, querier.queryRowArgs)
	}

	querier.row = scanRowFunc(func(dest ...any) error {
		for index, value := range []int{10, 2, 1, 25, 3} {
			*dest[index].(*int) = value
		}
		return nil
	})
	overview, err := repo.Overview(context.Background(), "2026-07-21", 50)
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.TotalStudents != 10 || overview.BlockedStudents != 2 || overview.QuotaExhaustedStudents != 1 || overview.RepliesToday != 25 || overview.RiskEventsToday != 3 {
		t.Fatalf("overview = %#v", overview)
	}
}

func TestAIRiskRepositoryListStudentsUsesUniformLimitAndFilters(t *testing.T) {
	lastReply := time.Date(2026, 7, 21, 4, 0, 0, 0, time.UTC)
	rows := &staticAIRiskRows{scans: []scanRowFunc{func(dest ...any) error {
		*dest[0].(*string) = "student-1"
		*dest[1].(*string) = "alice"
		*dest[2].(*string) = "alice@example.com"
		*dest[3].(*pgtype.Text) = pgtype.Text{String: "Alice", Valid: true}
		*dest[4].(*bool) = false
		*dest[5].(*pgtype.Text) = pgtype.Text{}
		*dest[6].(*pgtype.Timestamp) = pgtype.Timestamp{}
		*dest[7].(*int) = 50
		*dest[8].(*pgtype.Timestamp) = pgtype.Timestamp{Time: lastReply, Valid: true}
		*dest[9].(*int) = 1
		return nil
	}}}
	querier := &recordingAIRiskQuerier{rows: rows}
	repo, err := NewAIRiskRepository(querier)
	if err != nil {
		t.Fatalf("NewAIRiskRepository() error = %v", err)
	}
	items, total, err := repo.ListStudents(context.Background(), airiskapp.StudentListFilter{
		Page: 2, PageSize: 20, Search: "alice", Status: "quota_exhausted", UsageDate: "2026-07-21", DailyLimit: 50,
	})
	if err != nil {
		t.Fatalf("ListStudents() error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].DisplayName == nil || *items[0].DisplayName != "Alice" || items[0].LastAIReplyAt == nil {
		t.Fatalf("items=%#v total=%d", items, total)
	}
	for _, fragment := range []string{"usage_date = $1::date", "ILIKE $2", "replies_used >= $3", "LIMIT $4 OFFSET $5"} {
		if !strings.Contains(querier.querySQL, fragment) {
			t.Fatalf("student query missing %q: %s", fragment, querier.querySQL)
		}
	}
	wantArgs := []any{"2026-07-21", "%alice%", 50, 20, 20}
	for index := range wantArgs {
		if querier.queryArgs[index] != wantArgs[index] {
			t.Fatalf("student arg %d = %#v, want %#v", index, querier.queryArgs[index], wantArgs[index])
		}
	}
}

func TestAIRiskRepositoryUpsertSettingsCommitsTransaction(t *testing.T) {
	tx := &recordingAIRiskTx{}
	beginner := &recordingAIRiskBeginner{tx: tx}
	repo, err := NewAIRiskRepository(beginner)
	if err != nil {
		t.Fatalf("NewAIRiskRepository() error = %v", err)
	}
	now := time.Date(2026, 7, 21, 5, 0, 0, 0, time.UTC)
	updates := []airiskapp.SettingUpdate{
		{Key: airiskapp.DailyReplyLimitKey, Value: "80", Description: "daily", UpdatedAt: now},
		{Key: airiskapp.MaxConcurrencyKey, Value: "3", Description: "concurrency", UpdatedAt: now},
	}
	if err := repo.UpsertSettings(context.Background(), updates); err != nil {
		t.Fatalf("UpsertSettings() error = %v", err)
	}
	if beginner.begins != 1 || tx.commits != 1 || tx.rollbacks != 0 || len(tx.execSQL) != 2 {
		t.Fatalf("begins=%d commits=%d rollbacks=%d execs=%d", beginner.begins, tx.commits, tx.rollbacks, len(tx.execSQL))
	}
	for _, sql := range tx.execSQL {
		if !strings.Contains(sql, "ON CONFLICT (key) DO UPDATE") {
			t.Fatalf("upsert SQL = %s", sql)
		}
	}
}

func TestAIRiskRepositorySetStudentAccessCommitsControlAndEvent(t *testing.T) {
	now := time.Date(2026, 7, 21, 5, 30, 0, 0, time.UTC)
	tx := &recordingAIRiskTx{}
	tx.row = scanRowFunc(func(dest ...any) error {
		*dest[0].(*string) = "alice"
		*dest[1].(*string) = "STUDENT"
		return nil
	})
	beginner := &recordingAIRiskBeginner{tx: tx}
	repo, err := NewAIRiskRepository(beginner)
	if err != nil {
		t.Fatalf("NewAIRiskRepository() error = %v", err)
	}
	response, found, err := repo.SetStudentAccess(context.Background(), airiskapp.StudentAccessMutation{
		EventID: "event-1", StudentID: "student-1", ActorID: "admin-1", Blocked: true, Reason: "违规", EventDate: "2026-07-21", Now: now,
	})
	if err != nil || !found {
		t.Fatalf("SetStudentAccess() = %#v, %v, %v", response, found, err)
	}
	if !response.AIBlocked || response.BlockedReason != "违规" || response.BlockedAt == nil || !response.BlockedAt.Equal(now) {
		t.Fatalf("response = %#v", response)
	}
	if beginner.begins != 1 || tx.commits != 1 || tx.rollbacks != 0 || len(tx.execSQL) != 2 {
		t.Fatalf("begins=%d commits=%d rollbacks=%d execs=%d", beginner.begins, tx.commits, tx.rollbacks, len(tx.execSQL))
	}
	if !strings.Contains(tx.execSQL[0], "student_ai_access_controls") || !strings.Contains(tx.execSQL[1], "student_ai_risk_events") {
		t.Fatalf("exec SQL = %#v", tx.execSQL)
	}
	if tx.execArgs[1][3] != "admin_blocked" || tx.execArgs[1][4] != "warning" || tx.execArgs[1][5] != "access_blocked" {
		t.Fatalf("event args = %#v", tx.execArgs[1])
	}
}

func TestAIRiskRepositoryListsAndInsertsRiskEvents(t *testing.T) {
	createdAt := time.Date(2026, 7, 21, 6, 0, 0, 0, time.UTC)
	rows := &staticAIRiskRows{scans: []scanRowFunc{func(dest ...any) error {
		*dest[0].(*string) = "event-1"
		*dest[1].(*pgtype.Text) = pgtype.Text{String: "student-1", Valid: true}
		*dest[2].(*string) = "alice"
		*dest[3].(*string) = "content_blocked"
		*dest[4].(*string) = "critical"
		*dest[5].(*string) = "request_blocked"
		*dest[6].(*string) = "session_chat"
		*dest[7].(*string) = "代考"
		*dest[8].(*string) = "请帮我代考"
		*dest[9].(*pgtype.Text) = pgtype.Text{}
		*dest[10].(*time.Time) = createdAt
		return nil
	}}}
	querier := &recordingAIRiskQuerier{
		rows: rows,
		row: scanRowFunc(func(dest ...any) error {
			*dest[0].(*int) = 1
			return nil
		}),
	}
	repo, err := NewAIRiskRepository(querier)
	if err != nil {
		t.Fatalf("NewAIRiskRepository() error = %v", err)
	}
	items, total, err := repo.ListRiskEvents(context.Background(), airiskapp.EventListFilter{
		Page: 2, PageSize: 10, Search: "alice", EventType: "content_blocked",
	})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("ListRiskEvents() items=%#v total=%d error=%v", items, total, err)
	}
	if items[0].StudentID == nil || *items[0].StudentID != "student-1" || items[0].ActorID != nil {
		t.Fatalf("event = %#v", items[0])
	}
	for _, fragment := range []string{"ILIKE $1", "event_type = $2", "LIMIT $3 OFFSET $4"} {
		if !strings.Contains(querier.querySQL, fragment) {
			t.Fatalf("event query missing %q: %s", fragment, querier.querySQL)
		}
	}

	studentID := "student-1"
	event := airiskapp.RiskEvent{ID: "event-2", StudentID: &studentID, StudentUsername: "alice", EventType: "content_blocked", Severity: "critical", Action: "request_blocked", Source: "session_chat", MatchedRule: "代考", ContentExcerpt: "请帮我代考", ContentHash: "hash", EventDate: "2026-07-21", CreatedAt: createdAt}
	if err := repo.InsertRiskEvent(context.Background(), event); err != nil {
		t.Fatalf("InsertRiskEvent() error = %v", err)
	}
	if len(querier.execSQL) != 1 || !strings.Contains(querier.execSQL[0], "student_ai_risk_events") || len(querier.execArgs[0]) != 13 {
		t.Fatalf("insert SQL=%#v args=%#v", querier.execSQL, querier.execArgs)
	}
}

func TestAIRiskRepositoryPropagatesQueryErrors(t *testing.T) {
	wantErr := errors.New("query failed")
	querier := &recordingAIRiskQuerier{queryErr: wantErr}
	repo, err := NewAIRiskRepository(querier)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.GetSettings(context.Background(), []string{airiskapp.DailyReplyLimitKey})
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetSettings() error = %v, want %v", err, wantErr)
	}
}
