package session

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when the session is absent or not owned by the user.
var ErrNotFound = errors.New("session not found")

// Repository is the persistence surface required by session use cases.
type Repository interface {
	CreateSession(context.Context, LearningSession, Message) error
	GetSession(context.Context, string, string) (LearningSession, bool, error)
	InsertMessage(context.Context, Message) error
	ListMessages(context.Context, string, int, int) ([]Message, int, error)
	ListSessions(context.Context, string, int, int) ([]SessionListItem, int, error)
	EndSession(context.Context, string, string, time.Time) (EndState, bool, error)
	UpdateSessionTopic(context.Context, string, string, string) (string, bool, error)
	DeleteSession(context.Context, string, string) (bool, error)
	BatchDeleteSessions(context.Context, []string, string) (int, error)
}

// LearningSession stores one learning session.
type LearningSession struct {
	ID           string
	StudentID    string
	IsActive     bool
	CurrentTopic *string
	StartedAt    time.Time
	EndedAt      *time.Time
}

// Message stores one session message.
type Message struct {
	ID          string
	SessionID   string
	Role        string
	Content     string
	Agent       *string
	Attachments []string
	CreatedAt   time.Time
}

// SessionListItem stores a session row plus message count.
type SessionListItem struct {
	Session      LearningSession
	MessageCount int
}

// EndState identifies end-session update result.
type EndState string

const (
	// EndStateEnded means the session was active and has just been ended.
	EndStateEnded EndState = "ended"
	// EndStateAlreadyEnded means the session was already inactive.
	EndStateAlreadyEnded EndState = "already_ended"
)

// CreateSessionResponse is the Python-compatible POST /session/start response.
type CreateSessionResponse struct {
	SessionID      string          `json:"session_id"`
	UserID         string          `json:"user_id"`
	Topic          *string         `json:"topic"`
	Mode           string          `json:"mode"`
	Status         string          `json:"status"`
	CreatedAt      string          `json:"created_at"`
	WelcomeMessage MessageResponse `json:"welcome_message"`
}

// MessageResponse stores public message data.
type MessageResponse struct {
	ID          string   `json:"id"`
	Role        string   `json:"role"`
	Content     string   `json:"content"`
	Agent       *string  `json:"agent"`
	Timestamp   string   `json:"timestamp"`
	Attachments []string `json:"attachments"`
}

// HistoryResponse is the Python-compatible GET /session/{id}/history response.
type HistoryResponse struct {
	Messages []MessageResponse `json:"messages"`
	Total    int               `json:"total"`
	HasMore  bool              `json:"has_more"`
}

// SessionListResponse is the Python-compatible GET /session/list response.
type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	Total    int               `json:"total"`
}

// SessionResponse stores one list row.
type SessionResponse struct {
	SessionID    string  `json:"session_id"`
	UserID       string  `json:"user_id"`
	Topic        *string `json:"topic"`
	Status       string  `json:"status"`
	StartedAt    string  `json:"started_at"`
	EndedAt      *string `json:"ended_at"`
	MessageCount int     `json:"message_count"`
}

// EndResponse is the Python-compatible POST /session/{id}/end response.
type EndResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// UpdateModeResponse is the Python-compatible PATCH /session/{id}/mode response.
type UpdateModeResponse struct {
	SessionID string  `json:"session_id"`
	Mode      string  `json:"mode"`
	Topic     *string `json:"topic"`
}

// DeleteResponse is the Python-compatible DELETE /session/{id} response.
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// BatchDeleteResponse is the Python-compatible POST /session/batch-delete response.
type BatchDeleteResponse struct {
	Success      bool   `json:"success"`
	DeletedCount int    `json:"deleted_count"`
	Message      string `json:"message"`
}

// ChatResult stores the immediate SSE fallback response data.
type ChatResult struct {
	TaskID    string
	MessageID string
	Agent     string
	Content   string
}

// ChatAgent generates assistant responses for a learning session.
type ChatAgent interface {
	Generate(context.Context, ChatAgentInput) (ChatAgentOutput, error)
}

// ChatAgentInput carries session context into the configured agent runtime.
type ChatAgentInput struct {
	SessionID   string
	StudentID   string
	Message     string
	Attachments []string
	History     []Message
}

// ChatAgentOutput stores the generated assistant message.
type ChatAgentOutput struct {
	Agent   string
	Content string
}

// CancelTaskResponse is the Python-compatible task cancellation response.
type CancelTaskResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Service implements session use cases.
type Service struct {
	repo  Repository
	agent ChatAgent
	now   func() time.Time
	newID func() (string, error)
}

// Option customizes the session service.
type Option func(*Service)

// WithChatAgent enables AI-backed chat generation for session messages.
func WithChatAgent(agent ChatAgent) Option {
	return func(service *Service) {
		service.agent = agent
	}
}

// NewService creates a session service.
func NewService(repo Repository, options ...Option) (*Service, error) {
	if repo == nil {
		return nil, errors.New("session repository is nil")
	}
	service := &Service{repo: repo, now: time.Now, newID: NewUUID}
	for _, option := range options {
		option(service)
	}
	return service, nil
}

// CreateSession creates a learning session and welcome message.
func (s *Service) CreateSession(ctx context.Context, userID string, topic *string, mode string) (CreateSessionResponse, error) {
	if mode == "" {
		mode = "chat"
	}
	now := s.now()
	sessionID, err := s.newID()
	if err != nil {
		return CreateSessionResponse{}, err
	}
	messageID, err := s.newID()
	if err != nil {
		return CreateSessionResponse{}, err
	}
	agent := "tutor"
	session := LearningSession{
		ID:           sessionID,
		StudentID:    userID,
		IsActive:     true,
		CurrentTopic: topic,
		StartedAt:    now,
	}
	welcome := Message{
		ID:        messageID,
		SessionID: sessionID,
		Role:      "assistant",
		Content:   welcomeMessage(mode),
		Agent:     &agent,
		CreatedAt: now,
	}
	if err := s.repo.CreateSession(ctx, session, welcome); err != nil {
		return CreateSessionResponse{}, err
	}
	return CreateSessionResponse{
		SessionID: sessionID,
		UserID:    userID,
		Topic:     topic,
		Mode:      mode,
		Status:    "active",
		CreatedAt: formatTime(now),
		WelcomeMessage: MessageResponse{
			ID:          messageID,
			Role:        "assistant",
			Content:     welcome.Content,
			Agent:       &agent,
			Timestamp:   formatTime(now),
			Attachments: []string{},
		},
	}, nil
}

// ProcessChat stores the user message and generates a compatible assistant SSE payload.
func (s *Service) ProcessChat(ctx context.Context, sessionID string, userID string, message string, attachments []string) (ChatResult, error) {
	current, ok, err := s.repo.GetSession(ctx, sessionID, userID)
	if err != nil {
		return ChatResult{}, err
	}
	if !ok || !current.IsActive {
		return ChatResult{}, ErrNotFound
	}
	history, err := s.recentHistory(ctx, sessionID)
	if err != nil {
		return ChatResult{}, err
	}
	now := s.now()
	userMessageID, err := s.newID()
	if err != nil {
		return ChatResult{}, err
	}
	assistantMessageID, err := s.newID()
	if err != nil {
		return ChatResult{}, err
	}
	taskID, err := s.newID()
	if err != nil {
		return ChatResult{}, err
	}
	if err := s.repo.InsertMessage(ctx, Message{
		ID:          userMessageID,
		SessionID:   sessionID,
		Role:        "user",
		Content:     message,
		Attachments: attachments,
		CreatedAt:   now,
	}); err != nil {
		return ChatResult{}, err
	}
	output, err := s.generateAssistant(ctx, ChatAgentInput{
		SessionID:   sessionID,
		StudentID:   userID,
		Message:     message,
		Attachments: attachments,
		History:     history,
	})
	if err != nil {
		return ChatResult{}, err
	}
	agent := output.Agent
	if agent == "" {
		agent = "tutor"
	}
	if err := s.repo.InsertMessage(ctx, Message{
		ID:        assistantMessageID,
		SessionID: sessionID,
		Role:      "assistant",
		Content:   output.Content,
		Agent:     &agent,
		CreatedAt: now,
	}); err != nil {
		return ChatResult{}, err
	}
	return ChatResult{TaskID: taskID, MessageID: assistantMessageID, Agent: agent, Content: output.Content}, nil
}

func (s *Service) recentHistory(ctx context.Context, sessionID string) ([]Message, error) {
	const limit = 30
	messages, total, err := s.repo.ListMessages(ctx, sessionID, limit, 0)
	if err != nil {
		return nil, err
	}
	if total <= limit {
		return messages, nil
	}
	messages, _, err = s.repo.ListMessages(ctx, sessionID, limit, total-limit)
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *Service) generateAssistant(ctx context.Context, input ChatAgentInput) (ChatAgentOutput, error) {
	if s.agent == nil {
		return ChatAgentOutput{
			Agent:   "tutor",
			Content: "智能导师尚未配置；你的消息已保存。请管理员在 AI 模型设置中配置导师智能体，或在后端配置 EINO_ENABLED、EINO_API_KEY 和 EINO_MODEL 后恢复回复。",
		}, nil
	}
	output, err := s.agent.Generate(ctx, input)
	if err != nil {
		return ChatAgentOutput{
			Agent:   "tutor",
			Content: "智能导师暂时不可用；你的消息已保存。请稍后重试，或联系管理员检查导师智能体模型配置。",
		}, nil
	}
	if output.Agent == "" {
		output.Agent = "tutor"
	}
	if output.Content == "" {
		output.Content = "智能导师暂未生成有效回复，请稍后重试。"
	}
	return output, nil
}

// GetHistory returns a page of session messages.
func (s *Service) GetHistory(ctx context.Context, sessionID string, userID string, limit int, offset int) (HistoryResponse, error) {
	if _, ok, err := s.repo.GetSession(ctx, sessionID, userID); err != nil {
		return HistoryResponse{}, err
	} else if !ok {
		return HistoryResponse{Messages: []MessageResponse{}, Total: 0, HasMore: false}, nil
	}
	limit = clampInt(limit, 1, 100, 50)
	if offset < 0 {
		offset = 0
	}
	messages, total, err := s.repo.ListMessages(ctx, sessionID, limit, offset)
	if err != nil {
		return HistoryResponse{}, err
	}
	return HistoryResponse{
		Messages: toMessageResponses(messages),
		Total:    total,
		HasMore:  offset+limit < total,
	}, nil
}

// GetSessions returns the user's session list.
func (s *Service) GetSessions(ctx context.Context, userID string, limit int, offset int) (SessionListResponse, error) {
	limit = clampInt(limit, 1, 50, 20)
	if offset < 0 {
		offset = 0
	}
	rows, total, err := s.repo.ListSessions(ctx, userID, limit, offset)
	if err != nil {
		return SessionListResponse{}, err
	}
	sessions := make([]SessionResponse, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, toSessionResponse(row))
	}
	return SessionListResponse{Sessions: sessions, Total: total}, nil
}

// EndSession marks a session inactive.
func (s *Service) EndSession(ctx context.Context, sessionID string, userID string) (EndResponse, error) {
	state, ok, err := s.repo.EndSession(ctx, sessionID, userID, s.now())
	if err != nil {
		return EndResponse{}, err
	}
	if !ok {
		return EndResponse{}, ErrNotFound
	}
	if state == EndStateAlreadyEnded {
		return EndResponse{Status: "already_ended", Message: "会话已结束"}, nil
	}
	return EndResponse{Status: "ended", Message: "会话已成功结束"}, nil
}

// UpdateSessionMode updates the session topic to the mode label.
func (s *Service) UpdateSessionMode(ctx context.Context, sessionID string, userID string, mode string) (UpdateModeResponse, error) {
	topicValue := modeTopic(mode)
	topic, ok, err := s.repo.UpdateSessionTopic(ctx, sessionID, userID, topicValue)
	if err != nil {
		return UpdateModeResponse{}, err
	}
	if !ok {
		return UpdateModeResponse{}, ErrNotFound
	}
	return UpdateModeResponse{SessionID: sessionID, Mode: mode, Topic: &topic}, nil
}

// DeleteSession deletes one owned session.
func (s *Service) DeleteSession(ctx context.Context, sessionID string, userID string) (DeleteResponse, error) {
	ok, err := s.repo.DeleteSession(ctx, sessionID, userID)
	if err != nil {
		return DeleteResponse{}, err
	}
	if !ok {
		return DeleteResponse{Success: false, Message: "会话不存在或无权删除"}, nil
	}
	return DeleteResponse{Success: true, Message: "会话已删除"}, nil
}

// BatchDeleteSessions deletes owned sessions from the requested list.
func (s *Service) BatchDeleteSessions(ctx context.Context, sessionIDs []string, userID string) (BatchDeleteResponse, error) {
	if len(sessionIDs) == 0 {
		return BatchDeleteResponse{Success: false, DeletedCount: 0, Message: "没有找到可删除的会话"}, nil
	}
	count, err := s.repo.BatchDeleteSessions(ctx, sessionIDs, userID)
	if err != nil {
		return BatchDeleteResponse{}, err
	}
	if count == 0 {
		return BatchDeleteResponse{Success: false, DeletedCount: 0, Message: "没有找到可删除的会话"}, nil
	}
	return BatchDeleteResponse{Success: true, DeletedCount: count, Message: "成功删除 " + intString(count) + " 个会话"}, nil
}

// CancelTask returns a compatible response for non-resident Go task state.
func (s *Service) CancelTask(context.Context, string, string) (CancelTaskResponse, error) {
	return CancelTaskResponse{Success: false, Message: "任务不存在或已完成"}, nil
}

func toMessageResponses(messages []Message) []MessageResponse {
	responses := make([]MessageResponse, 0, len(messages))
	for _, message := range messages {
		responses = append(responses, MessageResponse{
			ID:          message.ID,
			Role:        message.Role,
			Content:     message.Content,
			Agent:       copyOptionalString(message.Agent),
			Timestamp:   formatTime(message.CreatedAt),
			Attachments: copyStrings(message.Attachments),
		})
	}
	return responses
}

func toSessionResponse(row SessionListItem) SessionResponse {
	session := row.Session
	return SessionResponse{
		SessionID:    session.ID,
		UserID:       session.StudentID,
		Topic:        copyOptionalString(session.CurrentTopic),
		Status:       sessionStatus(session.IsActive),
		StartedAt:    formatTime(session.StartedAt),
		EndedAt:      optionalFormattedTime(session.EndedAt),
		MessageCount: row.MessageCount,
	}
}

func sessionStatus(active bool) string {
	if active {
		return "active"
	}
	return "completed"
}

func welcomeMessage(mode string) string {
	switch mode {
	case "study":
		return "你好！我是你的 AI 高数学习助手。在学习模式下，我会系统性地引导你学习数学概念，从基础到进阶，确保你理解每个知识点。现在，你想学习什么主题？"
	case "practice":
		return "你好！欢迎进入练习模式！我会根据你的学习进度推荐适合的题目，并在你做题过程中提供实时反馈。准备好开始练习了吗？请告诉我你想练习的知识点。"
	case "explain":
		return "你好！在讲解模式下，我会对数学概念进行深入、详细的讲解，帮助你从本质上理解问题。请告诉我你想深入了解的主题或遇到的困惑。"
	default:
		return "你好！我是你的 AI 高数辅导助手。在聊天模式下，你可以随时问我任何数学问题，我会尽力给你最清晰的解答。有什么想问的吗？"
	}
}

func modeTopic(mode string) string {
	switch mode {
	case "study":
		return "学习模式"
	case "practice":
		return "练习模式"
	case "explain":
		return "讲解模式"
	case "chat":
		return "聊天模式"
	default:
		return mode
	}
}

func formatTime(value time.Time) string {
	return value.Format("2006-01-02T15:04:05.999999")
}

func optionalFormattedTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func copyOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func copyStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	result := make([]string, len(values))
	copy(result, values)
	return result
}

func clampInt(value int, minValue int, maxValue int, fallback int) int {
	if value == 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	current := value
	for current > 0 {
		digits = append([]byte{byte('0' + current%10)}, digits...)
		current /= 10
	}
	return string(digits)
}
