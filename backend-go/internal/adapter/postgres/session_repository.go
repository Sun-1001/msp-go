package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sessionapp "mathstudy/backend-go/internal/application/session"
)

// SessionRepository persists learning sessions and messages in PostgreSQL.
type SessionRepository struct {
	Repository
}

// NewSessionRepository creates a PostgreSQL-backed session repository.
func NewSessionRepository(db Querier) (SessionRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return SessionRepository{}, err
	}
	return SessionRepository{Repository: base}, nil
}

// CreateSession inserts a session and its welcome message.
func (r SessionRepository) CreateSession(ctx context.Context, session sessionapp.LearningSession, welcome sessionapp.Message) error {
	_, err := r.DB().Exec(ctx, `
		INSERT INTO public.learning_sessions (
			id,
			student_id,
			is_active,
			current_topic,
			current_content_id,
			contents_attempted,
			concepts_discussed,
			started_at,
			ended_at
		)
		VALUES ($1, $2, $3, $4, NULL, '[]'::json, '[]'::json, $5, NULL)`,
		session.ID,
		session.StudentID,
		session.IsActive,
		session.CurrentTopic,
		session.StartedAt,
	)
	if err != nil {
		return err
	}
	return r.InsertMessage(ctx, welcome)
}

// GetSession returns one owned session.
func (r SessionRepository) GetSession(ctx context.Context, sessionID string, userID string) (sessionapp.LearningSession, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT id, student_id, is_active, current_topic, started_at, ended_at
		FROM public.learning_sessions
		WHERE id = $1 AND student_id = $2`,
		sessionID,
		userID,
	)
	return scanOptionalSession(row)
}

// InsertMessage inserts one session message.
func (r SessionRepository) InsertMessage(ctx context.Context, message sessionapp.Message) error {
	attachmentsRaw, err := json.Marshal(message.Attachments)
	if err != nil {
		return err
	}
	_, err = r.DB().Exec(ctx, `
		INSERT INTO public.session_messages (
			id,
			session_id,
			role,
			content,
			agent_type,
			attachments,
			related_concept_ids,
			related_content_id,
			created_at
		)
		VALUES ($1, $2, $3::public.messagerole, $4, $5::public.agenttype, $6::json, '[]'::json, NULL, $7)`,
		message.ID,
		message.SessionID,
		roleToDB(message.Role),
		message.Content,
		agentToDB(message.Agent),
		string(attachmentsRaw),
		message.CreatedAt,
	)
	return err
}

// ListMessages returns session messages in ascending chronological order.
func (r SessionRepository) ListMessages(ctx context.Context, sessionID string, limit int, offset int) ([]sessionapp.Message, int, error) {
	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT count(id)::int
		FROM public.session_messages
		WHERE session_id = $1`,
		sessionID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB().Query(ctx, `
		SELECT id, session_id, role::text, content, agent_type::text, attachments, created_at
		FROM public.session_messages
		WHERE session_id = $1
		ORDER BY created_at ASC, id ASC
		OFFSET $2
		LIMIT $3`,
		sessionID,
		offset,
		limit,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	messages := []sessionapp.Message{}
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, 0, err
		}
		messages = append(messages, message)
	}
	return messages, total, rows.Err()
}

// ListSessions returns sessions with message counts.
func (r SessionRepository) ListSessions(ctx context.Context, userID string, limit int, offset int) ([]sessionapp.SessionListItem, int, error) {
	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT count(id)::int
		FROM public.learning_sessions
		WHERE student_id = $1`,
		userID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB().Query(ctx, `
		SELECT
			ls.id,
			ls.student_id,
			ls.is_active,
			ls.current_topic,
			ls.started_at,
			ls.ended_at,
			coalesce(count(sm.id), 0)::int AS message_count
		FROM public.learning_sessions ls
		LEFT JOIN public.session_messages sm ON sm.session_id = ls.id
		WHERE ls.student_id = $1
		GROUP BY ls.id, ls.student_id, ls.is_active, ls.current_topic, ls.started_at, ls.ended_at
		ORDER BY ls.started_at DESC, ls.id DESC
		OFFSET $2
		LIMIT $3`,
		userID,
		offset,
		limit,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []sessionapp.SessionListItem{}
	for rows.Next() {
		session, count, err := scanSessionListItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, sessionapp.SessionListItem{Session: session, MessageCount: count})
	}
	return items, total, rows.Err()
}

// EndSession marks a session inactive.
func (r SessionRepository) EndSession(ctx context.Context, sessionID string, userID string, endedAt time.Time) (sessionapp.EndState, bool, error) {
	session, ok, err := r.GetSession(ctx, sessionID, userID)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	if !session.IsActive {
		return sessionapp.EndStateAlreadyEnded, true, nil
	}
	_, err = r.DB().Exec(ctx, `
		UPDATE public.learning_sessions
		SET is_active = false, ended_at = $3
		WHERE id = $1 AND student_id = $2`,
		sessionID,
		userID,
		endedAt,
	)
	if err != nil {
		return "", false, err
	}
	return sessionapp.EndStateEnded, true, nil
}

// UpdateSessionTopic updates the topic field for a session.
func (r SessionRepository) UpdateSessionTopic(ctx context.Context, sessionID string, userID string, topic string) (string, bool, error) {
	var updated string
	err := r.DB().QueryRow(ctx, `
		UPDATE public.learning_sessions
		SET current_topic = $3
		WHERE id = $1 AND student_id = $2
		RETURNING current_topic`,
		sessionID,
		userID,
		topic,
	).Scan(&updated)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return updated, true, nil
}

// DeleteSession deletes one owned session and its messages.
func (r SessionRepository) DeleteSession(ctx context.Context, sessionID string, userID string) (bool, error) {
	exists, err := r.Exists(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM public.learning_sessions
			WHERE id = $1 AND student_id = $2
		)`,
		sessionID,
		userID,
	)
	if err != nil || !exists {
		return false, err
	}
	if _, err := r.DB().Exec(ctx, `DELETE FROM public.session_messages WHERE session_id = $1`, sessionID); err != nil {
		return false, err
	}
	tag, err := r.DB().Exec(ctx, `DELETE FROM public.learning_sessions WHERE id = $1 AND student_id = $2`, sessionID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// BatchDeleteSessions deletes owned sessions and their messages.
func (r SessionRepository) BatchDeleteSessions(ctx context.Context, sessionIDs []string, userID string) (int, error) {
	if len(sessionIDs) == 0 {
		return 0, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT id
		FROM public.learning_sessions
		WHERE student_id = $1 AND id = ANY($2::varchar[])`,
		userID,
		sessionIDs,
	)
	if err != nil {
		return 0, err
	}
	validIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		validIDs = append(validIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(validIDs) == 0 {
		return 0, nil
	}
	if _, err := r.DB().Exec(ctx, `DELETE FROM public.session_messages WHERE session_id = ANY($1::varchar[])`, validIDs); err != nil {
		return 0, err
	}
	tag, err := r.DB().Exec(ctx, `DELETE FROM public.learning_sessions WHERE student_id = $1 AND id = ANY($2::varchar[])`, userID, validIDs)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func scanOptionalSession(row pgx.Row) (sessionapp.LearningSession, bool, error) {
	session, err := scanSession(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return sessionapp.LearningSession{}, false, nil
		}
		return sessionapp.LearningSession{}, false, err
	}
	return session, true, nil
}

func scanSession(scanner rowScanner) (sessionapp.LearningSession, error) {
	var session sessionapp.LearningSession
	var topic pgtype.Text
	var endedAt pgtype.Timestamp
	if err := scanner.Scan(&session.ID, &session.StudentID, &session.IsActive, &topic, &session.StartedAt, &endedAt); err != nil {
		return sessionapp.LearningSession{}, err
	}
	session.CurrentTopic = textPtr(topic)
	if endedAt.Valid {
		value := endedAt.Time
		session.EndedAt = &value
	}
	return session, nil
}

func scanSessionListItem(rows pgx.Rows) (sessionapp.LearningSession, int, error) {
	var session sessionapp.LearningSession
	var topic pgtype.Text
	var endedAt pgtype.Timestamp
	var count int
	if err := rows.Scan(&session.ID, &session.StudentID, &session.IsActive, &topic, &session.StartedAt, &endedAt, &count); err != nil {
		return sessionapp.LearningSession{}, 0, err
	}
	session.CurrentTopic = textPtr(topic)
	if endedAt.Valid {
		value := endedAt.Time
		session.EndedAt = &value
	}
	return session, count, nil
}

func scanMessage(rows pgx.Rows) (sessionapp.Message, error) {
	var message sessionapp.Message
	var agent pgtype.Text
	var attachmentsRaw []byte
	if err := rows.Scan(&message.ID, &message.SessionID, &message.Role, &message.Content, &agent, &attachmentsRaw, &message.CreatedAt); err != nil {
		return sessionapp.Message{}, err
	}
	message.Role = roleFromDB(message.Role)
	if agent.Valid {
		value := agentFromDB(agent.String)
		message.Agent = &value
	}
	attachments, err := decodeStringSlice(attachmentsRaw)
	if err != nil {
		return sessionapp.Message{}, fmt.Errorf("decode message attachments: %w", err)
	}
	message.Attachments = attachments
	return message, nil
}

func roleToDB(role string) string {
	switch role {
	case "assistant":
		return "ASSISTANT"
	case "system":
		return "SYSTEM"
	default:
		return "USER"
	}
}

func roleFromDB(role string) string {
	switch role {
	case "ASSISTANT":
		return "assistant"
	case "SYSTEM":
		return "system"
	default:
		return "user"
	}
}

func agentToDB(agent *string) any {
	if agent == nil {
		return nil
	}
	switch *agent {
	case "math_solver":
		return "SOLVER"
	case "diagnostician":
		return "DIAGNOSTICIAN"
	case "tutor":
		return "TUTOR"
	default:
		return nil
	}
}

func agentFromDB(agent string) string {
	switch agent {
	case "SOLVER":
		return "math_solver"
	case "DIAGNOSTICIAN":
		return "diagnostician"
	case "TUTOR":
		return "tutor"
	case "PLANNER":
		return "planner"
	default:
		return ""
	}
}
