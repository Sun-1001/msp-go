package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	sessionapp "mathstudy/backend-go/internal/application/session"
)

type recordingSessionQuerier struct {
	execSQL  string
	execArgs []any
	execErr  error
}

func (q *recordingSessionQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execSQL = sql
	q.execArgs = append([]any(nil), args...)
	return pgconn.CommandTag{}, q.execErr
}

func (*recordingSessionQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (*recordingSessionQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func TestSessionRepositoryInsertMeteredAssistantMessageUsesOneAtomicStatement(t *testing.T) {
	querier := &recordingSessionQuerier{}
	repo, err := NewSessionRepository(querier)
	if err != nil {
		t.Fatalf("NewSessionRepository() error = %v", err)
	}
	agent := "tutor"
	createdAt := time.Date(2026, 7, 21, 4, 5, 6, 0, time.UTC)
	message := sessionapp.Message{
		ID:          "message-1",
		SessionID:   "session-1",
		Role:        "assistant",
		Content:     "回复内容",
		Agent:       &agent,
		Attachments: []string{"/uploads/images/a.png"},
		CreatedAt:   createdAt,
	}

	err = repo.InsertMeteredAssistantMessage(context.Background(), "student-1", message, "2026-07-21")
	if err != nil {
		t.Fatalf("InsertMeteredAssistantMessage() error = %v", err)
	}
	for _, fragment := range []string{
		"WITH inserted_message AS",
		"INSERT INTO public.session_messages",
		"INSERT INTO public.student_ai_reply_usage",
		"FROM inserted_message",
	} {
		if !strings.Contains(querier.execSQL, fragment) {
			t.Fatalf("atomic insert SQL missing %q: %s", fragment, querier.execSQL)
		}
	}
	if len(querier.execArgs) != 9 {
		t.Fatalf("exec args = %#v", querier.execArgs)
	}
	wantArgs := []any{"message-1", "session-1", "ASSISTANT", "回复内容", "TUTOR", `["/uploads/images/a.png"]`, createdAt, "student-1", "2026-07-21"}
	for index := range wantArgs {
		if querier.execArgs[index] != wantArgs[index] {
			t.Fatalf("exec arg %d = %#v, want %#v", index, querier.execArgs[index], wantArgs[index])
		}
	}
}

func TestSessionRepositoryInsertMeteredAssistantMessageReturnsExecError(t *testing.T) {
	wantErr := errors.New("insert failed")
	querier := &recordingSessionQuerier{execErr: wantErr}
	repo, err := NewSessionRepository(querier)
	if err != nil {
		t.Fatalf("NewSessionRepository() error = %v", err)
	}

	err = repo.InsertMeteredAssistantMessage(context.Background(), "student-1", sessionapp.Message{ID: "message-1"}, "2026-07-21")
	if !errors.Is(err, wantErr) {
		t.Fatalf("InsertMeteredAssistantMessage() error = %v, want %v", err, wantErr)
	}
}
