package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateSessionStoresWelcomeMessage(t *testing.T) {
	now := time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC)
	repo := &fakeSessionRepo{}
	service := newTestService(repo, now, "session-1", "message-1")
	topic := "极限"

	response, err := service.CreateSession(context.Background(), "student-1", &topic, "study")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if response.SessionID != "session-1" || response.UserID != "student-1" || response.Mode != "study" {
		t.Fatalf("response = %#v", response)
	}
	if repo.createdSession.ID != "session-1" || repo.createdSession.CurrentTopic == nil || *repo.createdSession.CurrentTopic != "极限" {
		t.Fatalf("created session = %#v", repo.createdSession)
	}
	if repo.createdWelcome.ID != "message-1" || repo.createdWelcome.Role != "assistant" || repo.createdWelcome.Agent == nil || *repo.createdWelcome.Agent != "tutor" {
		t.Fatalf("welcome = %#v", repo.createdWelcome)
	}
}

func TestProcessChatFallbackStoresMessagesAndReturnsSSEData(t *testing.T) {
	now := time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC)
	repo := &fakeSessionRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: true},
		hasSession: true,
	}
	service := newTestService(repo, now, "user-msg", "ai-msg", "task-1")

	result, err := service.ProcessChatFallback(context.Background(), "session-1", "student-1", "你好", []string{"/uploads/a.png"})
	if err != nil {
		t.Fatalf("ProcessChatFallback() error = %v", err)
	}
	if result.TaskID != "task-1" || result.MessageID != "ai-msg" || result.Agent != "tutor" || result.Content == "" {
		t.Fatalf("result = %#v", result)
	}
	if len(repo.insertedMessages) != 2 {
		t.Fatalf("messages = %#v", repo.insertedMessages)
	}
	if repo.insertedMessages[0].Role != "user" || repo.insertedMessages[0].Attachments[0] != "/uploads/a.png" {
		t.Fatalf("user message = %#v", repo.insertedMessages[0])
	}
	if repo.insertedMessages[1].Role != "assistant" {
		t.Fatalf("assistant message = %#v", repo.insertedMessages[1])
	}
}

func TestProcessChatFallbackRejectsInactiveSession(t *testing.T) {
	repo := &fakeSessionRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: false},
		hasSession: true,
	}
	service := newTestService(repo, time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC), "id")

	_, err := service.ProcessChatFallback(context.Background(), "session-1", "student-1", "你好", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ProcessChatFallback() error = %v, want ErrNotFound", err)
	}
}

func TestGetHistoryClampsPaginationAndBuildsHasMore(t *testing.T) {
	repo := &fakeSessionRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: true},
		hasSession: true,
		messages: []Message{
			{ID: "m1", Role: "assistant", Content: "hello", CreatedAt: time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC)},
		},
		messageTotal: 3,
	}
	service := newTestService(repo, time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC), "id")

	response, err := service.GetHistory(context.Background(), "session-1", "student-1", 1, 1)
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}
	if response.Total != 3 || !response.HasMore || len(response.Messages) != 1 {
		t.Fatalf("response = %#v", response)
	}
	if repo.lastMessageLimit != 1 || repo.lastMessageOffset != 1 {
		t.Fatalf("pagination = %d/%d", repo.lastMessageLimit, repo.lastMessageOffset)
	}
}

func TestGetSessionsAndUpdateMode(t *testing.T) {
	topic := "原主题"
	repo := &fakeSessionRepo{
		sessionItems: []SessionListItem{
			{
				Session:      LearningSession{ID: "session-1", StudentID: "student-1", IsActive: true, CurrentTopic: &topic, StartedAt: time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC)},
				MessageCount: 2,
			},
		},
		sessionTotal: 1,
		updatedTopic: "讲解模式",
		updateOK:     true,
	}
	service := newTestService(repo, time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC), "id")

	list, err := service.GetSessions(context.Background(), "student-1", 20, 0)
	if err != nil {
		t.Fatalf("GetSessions() error = %v", err)
	}
	if list.Total != 1 || len(list.Sessions) != 1 || list.Sessions[0].MessageCount != 2 {
		t.Fatalf("list = %#v", list)
	}

	mode, err := service.UpdateSessionMode(context.Background(), "session-1", "student-1", "explain")
	if err != nil {
		t.Fatalf("UpdateSessionMode() error = %v", err)
	}
	if mode.Topic == nil || *mode.Topic != "讲解模式" {
		t.Fatalf("mode = %#v", mode)
	}
}

func TestEndDeleteAndBatchDeleteResponses(t *testing.T) {
	repo := &fakeSessionRepo{
		endState:     EndStateAlreadyEnded,
		endOK:        true,
		deleteOK:     true,
		batchDeleted: 2,
	}
	service := newTestService(repo, time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC), "id")

	end, err := service.EndSession(context.Background(), "session-1", "student-1")
	if err != nil {
		t.Fatalf("EndSession() error = %v", err)
	}
	if end.Status != "already_ended" {
		t.Fatalf("end = %#v", end)
	}
	deleted, err := service.DeleteSession(context.Background(), "session-1", "student-1")
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if !deleted.Success {
		t.Fatalf("delete = %#v", deleted)
	}
	batch, err := service.BatchDeleteSessions(context.Background(), []string{"a", "b"}, "student-1")
	if err != nil {
		t.Fatalf("BatchDeleteSessions() error = %v", err)
	}
	if !batch.Success || batch.DeletedCount != 2 || batch.Message != "成功删除 2 个会话" {
		t.Fatalf("batch = %#v", batch)
	}
}

func newTestService(repo Repository, now time.Time, ids ...string) *Service {
	service, err := NewService(repo)
	if err != nil {
		panic(err)
	}
	service.now = func() time.Time { return now }
	service.newID = sequentialIDs(ids...)
	return service
}

func sequentialIDs(values ...string) func() (string, error) {
	index := 0
	return func() (string, error) {
		if index >= len(values) {
			return "extra-id", nil
		}
		value := values[index]
		index++
		return value, nil
	}
}

type fakeSessionRepo struct {
	createdSession    LearningSession
	createdWelcome    Message
	session           LearningSession
	hasSession        bool
	insertedMessages  []Message
	messages          []Message
	messageTotal      int
	lastMessageLimit  int
	lastMessageOffset int
	sessionItems      []SessionListItem
	sessionTotal      int
	endState          EndState
	endOK             bool
	updatedTopic      string
	updateOK          bool
	deleteOK          bool
	batchDeleted      int
}

func (r *fakeSessionRepo) CreateSession(_ context.Context, session LearningSession, welcome Message) error {
	r.createdSession = session
	r.createdWelcome = welcome
	return nil
}

func (r *fakeSessionRepo) GetSession(context.Context, string, string) (LearningSession, bool, error) {
	return r.session, r.hasSession, nil
}

func (r *fakeSessionRepo) InsertMessage(_ context.Context, message Message) error {
	r.insertedMessages = append(r.insertedMessages, message)
	return nil
}

func (r *fakeSessionRepo) ListMessages(_ context.Context, _ string, limit int, offset int) ([]Message, int, error) {
	r.lastMessageLimit = limit
	r.lastMessageOffset = offset
	return r.messages, r.messageTotal, nil
}

func (r *fakeSessionRepo) ListSessions(context.Context, string, int, int) ([]SessionListItem, int, error) {
	return r.sessionItems, r.sessionTotal, nil
}

func (r *fakeSessionRepo) EndSession(context.Context, string, string, time.Time) (EndState, bool, error) {
	return r.endState, r.endOK, nil
}

func (r *fakeSessionRepo) UpdateSessionTopic(context.Context, string, string, string) (string, bool, error) {
	return r.updatedTopic, r.updateOK, nil
}

func (r *fakeSessionRepo) DeleteSession(context.Context, string, string) (bool, error) {
	return r.deleteOK, nil
}

func (r *fakeSessionRepo) BatchDeleteSessions(context.Context, []string, string) (int, error) {
	return r.batchDeleted, nil
}
