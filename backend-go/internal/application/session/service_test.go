package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	airiskapp "mathstudy/backend-go/internal/application/airisk"
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

func TestProcessChatUsesConfiguredAgentAndReturnsSSEData(t *testing.T) {
	now := time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC)
	repo := &fakeSessionRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: true},
		hasSession: true,
		messages: []Message{
			{ID: "history-1", Role: "assistant", Content: "上一条建议", CreatedAt: now.Add(-time.Minute)},
		},
		messageTotal: 1,
	}
	service := newTestService(repo, now, "user-msg", "ai-msg", "task-1")
	service.agent = fakeChatAgent{output: ChatAgentOutput{Agent: "tutor", Content: "Eino 回复"}}
	lease := &fakeAILease{}
	guard := &fakeAIGuard{lease: lease}
	WithAIRequestGuard(guard)(service)

	result, err := service.ProcessChat(context.Background(), "session-1", "student-1", "你好", []string{"/uploads/images/a.png"})
	if err != nil {
		t.Fatalf("ProcessChat() error = %v", err)
	}
	if result.TaskID != "task-1" || result.MessageID != "ai-msg" || result.Agent != "tutor" || result.Content != "Eino 回复" {
		t.Fatalf("result = %#v", result)
	}
	if len(repo.insertedMessages) != 2 {
		t.Fatalf("messages = %#v", repo.insertedMessages)
	}
	if repo.insertedMessages[0].Role != "user" || repo.insertedMessages[0].Attachments[0] != "/uploads/images/a.png" {
		t.Fatalf("user message = %#v", repo.insertedMessages[0])
	}
	if repo.insertedMessages[1].Role != "assistant" || repo.insertedMessages[1].Content != "Eino 回复" {
		t.Fatalf("assistant message = %#v", repo.insertedMessages[1])
	}
	if repo.meteredMessages != 1 || repo.meteredStudentID != "student-1" || repo.meteredUsageDate != "2026-04-26" {
		t.Fatalf("metered reply = count=%d student=%q date=%q", repo.meteredMessages, repo.meteredStudentID, repo.meteredUsageDate)
	}
	if guard.source != "session_chat" || guard.content != "你好" || !guard.metered || lease.releases != 1 {
		t.Fatalf("guard=%#v lease releases=%d", guard, lease.releases)
	}
}

func TestProcessChatStopsBeforePersistenceWhenGuardRejects(t *testing.T) {
	repo := &fakeSessionRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: true},
		hasSession: true,
	}
	service := newTestService(repo, time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC), "user-msg")
	service.guard = &fakeAIGuard{err: airiskapp.ErrContentBlocked}

	_, err := service.ProcessChat(context.Background(), "session-1", "student-1", "违规内容", nil)
	if !errors.Is(err, airiskapp.ErrContentBlocked) {
		t.Fatalf("ProcessChat() error = %v", err)
	}
	if len(repo.insertedMessages) != 0 {
		t.Fatalf("inserted messages = %#v", repo.insertedMessages)
	}
}

func TestProcessChatRejectsUnsafeAttachments(t *testing.T) {
	cases := []struct {
		name        string
		attachments []string
	}{
		{name: "external url", attachments: []string{"https://example.com/a.png"}},
		{name: "document path", attachments: []string{"/uploads/documents/a.pdf"}},
		{name: "path traversal", attachments: []string{"/uploads/images/../documents/a.pdf"}},
		{name: "query string", attachments: []string{"/uploads/images/a.png?download=1"}},
		{name: "encoded traversal", attachments: []string{"/uploads/images/%2e%2e/a.png"}},
		{name: "too many", attachments: []string{
			"/uploads/images/1.png",
			"/uploads/images/2.png",
			"/uploads/images/3.png",
			"/uploads/images/4.png",
			"/uploads/images/5.png",
			"/uploads/images/6.png",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeSessionRepo{
				session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: true},
				hasSession: true,
			}
			service := newTestService(repo, time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC), "user-msg", "ai-msg", "task-1")

			_, err := service.ProcessChat(context.Background(), "session-1", "student-1", "你好", tc.attachments)
			if !errors.Is(err, ErrInvalidAttachment) {
				t.Fatalf("ProcessChat() error = %v, want ErrInvalidAttachment", err)
			}
			if len(repo.insertedMessages) != 0 {
				t.Fatalf("inserted messages = %#v, want none", repo.insertedMessages)
			}
		})
	}
}

func TestProcessChatFallsBackWhenAgentIsNotConfigured(t *testing.T) {
	now := time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC)
	repo := &fakeSessionRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: true},
		hasSession: true,
	}
	service := newTestService(repo, now, "user-msg", "ai-msg", "task-1")

	result, err := service.ProcessChat(context.Background(), "session-1", "student-1", "你好", nil)
	if err != nil {
		t.Fatalf("ProcessChat() error = %v", err)
	}
	if result.Agent != "tutor" || result.Content == "" {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.Content, "智能导师尚未配置") {
		t.Fatalf("fallback content = %q", result.Content)
	}
	if repo.meteredMessages != 0 {
		t.Fatalf("fallback metered messages = %d", repo.meteredMessages)
	}
}

func TestProcessChatFallsBackWhenAgentFails(t *testing.T) {
	now := time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC)
	repo := &fakeSessionRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: true},
		hasSession: true,
	}
	service := newTestService(repo, now, "user-msg", "ai-msg", "task-1")
	service.agent = fakeChatAgent{err: errors.New("model unavailable")}

	result, err := service.ProcessChat(context.Background(), "session-1", "student-1", "你好", nil)
	if err != nil {
		t.Fatalf("ProcessChat() error = %v", err)
	}
	if result.Agent != "tutor" || !strings.Contains(result.Content, "智能导师暂时不可用") {
		t.Fatalf("result = %#v", result)
	}
	if len(repo.insertedMessages) != 2 || repo.insertedMessages[1].Role != "assistant" {
		t.Fatalf("messages = %#v", repo.insertedMessages)
	}
	if repo.meteredMessages != 0 {
		t.Fatalf("failed-agent fallback metered messages = %d", repo.meteredMessages)
	}
}

func TestProcessChatRejectsInactiveSession(t *testing.T) {
	repo := &fakeSessionRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", IsActive: false},
		hasSession: true,
	}
	service := newTestService(repo, time.Date(2026, time.April, 26, 9, 0, 0, 0, time.UTC), "id")

	_, err := service.ProcessChat(context.Background(), "session-1", "student-1", "你好", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ProcessChat() error = %v, want ErrNotFound", err)
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
	meteredMessages   int
	meteredStudentID  string
	meteredUsageDate  string
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

type fakeChatAgent struct {
	output ChatAgentOutput
	err    error
}

type fakeAIGuard struct {
	lease     airiskapp.Lease
	err       error
	studentID string
	source    string
	content   string
	metered   bool
}

type fakeAILease struct{ releases int }

func (l *fakeAILease) Release(context.Context) error {
	l.releases++
	return nil
}

func (g *fakeAIGuard) Acquire(_ context.Context, studentID, source, content string, metered bool) (airiskapp.Lease, error) {
	g.studentID = studentID
	g.source = source
	g.content = content
	g.metered = metered
	return g.lease, g.err
}

func (a fakeChatAgent) Generate(context.Context, ChatAgentInput) (ChatAgentOutput, error) {
	if a.err != nil {
		return ChatAgentOutput{}, a.err
	}
	return a.output, nil
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

func (r *fakeSessionRepo) InsertMeteredAssistantMessage(_ context.Context, studentID string, message Message, usageDate string) error {
	r.insertedMessages = append(r.insertedMessages, message)
	r.meteredMessages++
	r.meteredStudentID = studentID
	r.meteredUsageDate = usageDate
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
