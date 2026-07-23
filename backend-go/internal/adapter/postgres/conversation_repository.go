package postgres

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	conversationapp "mathstudy/backend-go/internal/application/conversation"
	"mathstudy/backend-go/internal/domain/user"
)

// ConversationRepository persists conversation data in PostgreSQL.
type ConversationRepository struct {
	Repository
}

// NewConversationRepository creates a PostgreSQL-backed conversation repository.
func NewConversationRepository(db Querier) (ConversationRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return ConversationRepository{}, err
	}
	return ConversationRepository{Repository: base}, nil
}

// ListConversations returns paginated conversations for a user.
func (r ConversationRepository) ListConversations(ctx context.Context, userID string, role user.Role, search string, status string, className string, page int, pageSize int) ([]conversationapp.ConversationItem, int, error) {
	pgPage, err := NewPage((page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var total int
	var items []conversationapp.ConversationItem

	if role == user.RoleStudent {
		items, total, err = r.listStudentConversations(ctx, userID, search, pgPage)
	} else {
		items, total, err = r.listTeacherConversations(ctx, userID, search, status, className, pgPage)
	}
	return items, total, err
}

func (r ConversationRepository) listStudentConversations(ctx context.Context, studentID string, search string, page Page) ([]conversationapp.ConversationItem, int, error) {
	args := []any{studentID}
	searchFilter := ""
	if strings.TrimSpace(search) != "" {
		searchFilter = ` AND (u.display_name ILIKE $2 OR u.username ILIKE $2 OR c.subject ILIKE $2)`
		args = append(args, "%"+search+"%")
	}

	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM public.conversations c
		JOIN public.users u ON u.id = c.teacher_id
		WHERE c.student_id = $1 AND c.is_archived = false`+searchFilter,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	countArgs := len(args)
	args = append(args, page.Limit, page.Offset)

	rows, err := r.DB().Query(ctx, `
		SELECT c.id, c.teacher_id, u.display_name, u.username, c.subject, c.last_message_at, c.is_archived,
			COALESCE(cnv.unread_count, 0),
			(SELECT text FROM public.conversation_messages WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1)
		FROM public.conversations c
		JOIN public.users u ON u.id = c.teacher_id
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS unread_count
			FROM public.conversation_messages cm
			WHERE cm.conversation_id = c.id AND cm.sender_role = 'teacher' AND cm.read_at IS NULL
		) cnv ON true
		WHERE c.student_id = $1 AND c.is_archived = false`+searchFilter+`
		ORDER BY c.last_message_at DESC
		LIMIT $`+pgIdx(countArgs+1)+` OFFSET $`+pgIdx(countArgs+2),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]conversationapp.ConversationItem, 0)
	for rows.Next() {
		var item conversationapp.ConversationItem
		var teacherName, teacherUsername, subject string
		var displayName, lastMsg pgtype.Text
		var unread int
		if err := rows.Scan(&item.ID, &item.TeacherID, &displayName, &teacherUsername, &subject, &item.LastTime, &item.Archived, &unread, &lastMsg); err != nil {
			return nil, 0, err
		}
		if displayName.Valid {
			teacherName = displayName.String
		} else {
			teacherName = teacherUsername
		}
		item.TeacherName = teacherName
		item.Scope = subject
		if lastMsg.Valid {
			item.LastMessage = lastMsg.String
		}
		item.Unread = unread
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r ConversationRepository) listTeacherConversations(ctx context.Context, teacherID string, search string, status string, className string, page Page) ([]conversationapp.ConversationItem, int, error) {
	args := []any{teacherID}
	whereIdx := 2
	searchFilter := ""
	if strings.TrimSpace(search) != "" {
		searchFilter = ` AND (u.display_name ILIKE $` + idxStr(whereIdx) + ` OR u.username ILIKE $` + idxStr(whereIdx) + ` OR c.subject ILIKE $` + idxStr(whereIdx) + `)`
		args = append(args, "%"+search+"%")
		whereIdx++
	}
	if strings.TrimSpace(className) != "" {
		searchFilter += ` AND c.subject ILIKE $` + idxStr(whereIdx)
		args = append(args, "%"+className+"%")
		whereIdx++
	}

	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM public.conversations c
		JOIN public.users u ON u.id = c.student_id
		WHERE c.teacher_id = $1`+searchFilter,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	countArgs := len(args)
	args = append(args, page.Limit, page.Offset)

	rows, err := r.DB().Query(ctx, `
		SELECT c.id, c.student_id, u.display_name, u.username, c.subject, c.last_message_at,
			(SELECT text FROM public.conversation_messages WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1),
			EXISTS(SELECT 1 FROM public.conversation_messages cm WHERE cm.conversation_id = c.id AND cm.sender_role = 'student' AND cm.read_at IS NULL) AS unread,
			(SELECT cm2.sender_role = 'student' FROM public.conversation_messages cm2 WHERE cm2.conversation_id = c.id ORDER BY cm2.created_at DESC LIMIT 1) AS pending_reply
		FROM public.conversations c
		JOIN public.users u ON u.id = c.student_id
		WHERE c.teacher_id = $1`+searchFilter+`
		ORDER BY c.last_message_at DESC
		LIMIT $`+idxStr(countArgs+1)+` OFFSET $`+idxStr(countArgs+2),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]conversationapp.ConversationItem, 0)
	for rows.Next() {
		var item conversationapp.ConversationItem
		var studentName, studentUsername, subject string
		var displayName, lastMsg pgtype.Text
		var unread, pendingReply bool
		if err := rows.Scan(&item.ID, &item.StudentID, &displayName, &studentUsername, &subject, &item.LastTime, &lastMsg, &unread, &pendingReply); err != nil {
			return nil, 0, err
		}
		if displayName.Valid {
			studentName = displayName.String
		} else {
			studentName = studentUsername
		}
		item.StudentName = studentName
		item.ClassName = subject
		if lastMsg.Valid {
			item.LastMessage = lastMsg.String
		}
		if unread {
			item.Unread = 1
		}
		item.PendingReply = pendingReply
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// GetConversation returns a conversation with full message history.
func (r ConversationRepository) GetConversation(ctx context.Context, conversationID string, userID string, page, pageSize int) (conversationapp.ConversationDetail, bool, error) {
	var detail conversationapp.ConversationDetail
	var teacherName, teacherUsername, studentName, studentUsername, subject string
	var teacherDisplay, studentDisplay pgtype.Text
	err := r.DB().QueryRow(ctx, `
		SELECT c.id, c.subject, c.last_message_at, c.is_archived,
			t.display_name, t.username,
			s.display_name, s.username
		FROM public.conversations c
		JOIN public.users t ON t.id = c.teacher_id
		JOIN public.users s ON s.id = c.student_id
		WHERE c.id = $1 AND (c.student_id = $2 OR c.teacher_id = $2)`,
		conversationID, userID,
	).Scan(&detail.ID, &subject, &detail.LastTime, &detail.Archived,
		&teacherDisplay, &teacherUsername,
		&studentDisplay, &studentUsername)
	if err != nil {
		if err == pgx.ErrNoRows {
			return conversationapp.ConversationDetail{}, false, nil
		}
		return conversationapp.ConversationDetail{}, false, err
	}
	if teacherDisplay.Valid {
		teacherName = teacherDisplay.String
	} else {
		teacherName = teacherUsername
	}
	if studentDisplay.Valid {
		studentName = studentDisplay.String
	} else {
		studentName = studentUsername
	}
	detail.TeacherName = teacherName
	detail.StudentName = studentName
	detail.Scope = subject
	detail.ClassName = subject

	if page < 1 { page = 1 }
	pgPage, err := NewPage((page-1)*pageSize, pageSize)
	if err != nil { return conversationapp.ConversationDetail{}, false, err }
	if err := r.DB().QueryRow(ctx, `SELECT COUNT(*) FROM public.conversation_messages WHERE conversation_id = $1`, conversationID).Scan(&detail.MessagesTotal); err != nil {
		return conversationapp.ConversationDetail{}, false, err
	}
	detail.MessagesPage, detail.MessagesSize = page, pgPage.Limit
	// Load the newest page, then restore chronological order for the UI.
	msgRows, err := r.DB().Query(ctx, `
		SELECT cm.id, cm.sender_role, cm.text, cm.created_at, cm.read_at
		FROM public.conversation_messages cm
		WHERE cm.conversation_id = $1
		ORDER BY cm.created_at DESC
		LIMIT $2 OFFSET $3`,
		conversationID, pgPage.Limit, pgPage.Offset,
	)
	if err != nil {
		return conversationapp.ConversationDetail{}, false, err
	}
	defer msgRows.Close()

	detail.Messages = make([]conversationapp.Message, 0)
	for msgRows.Next() {
		var msg conversationapp.Message
		var readAt pgtype.Timestamp
		if err := msgRows.Scan(&msg.ID, &msg.From, &msg.Text, &msg.Time, &readAt); err != nil {
			return conversationapp.ConversationDetail{}, false, err
		}
		if readAt.Valid {
			b := true
			msg.ReadByRecipient = &b
		}
		detail.Messages = append(detail.Messages, msg)
	}
	if err := msgRows.Err(); err != nil {
		return conversationapp.ConversationDetail{}, false, err
	}
	for left, right := 0, len(detail.Messages)-1; left < right; left, right = left+1, right-1 { detail.Messages[left], detail.Messages[right] = detail.Messages[right], detail.Messages[left] }

	return detail, true, nil
}

// MarkConversationRead marks messages from the other party as read.
func (r ConversationRepository) MarkConversationRead(ctx context.Context, conversationID string, userID string) error {
	_, err := r.DB().Exec(ctx, `
		UPDATE public.conversation_messages
		SET read_at = now()
		WHERE conversation_id = $1
		  AND sender_id != $2
		  AND read_at IS NULL
		  AND EXISTS (
			  SELECT 1 FROM public.conversations c
			  WHERE c.id = $1 AND (c.student_id = $2 OR c.teacher_id = $2)
		  )`,
		conversationID, userID,
	)
	return err
}

// CreateConversation creates a conversation and its first message.
func (r ConversationRepository) CreateConversation(ctx context.Context, creatorID string, creatorRole user.Role, targetID string, subject string, initialMessage string, now time.Time) (conversationapp.ConversationDetail, error) {
	if creatorRole != user.RoleStudent && creatorRole != user.RoleTeacher {
		return conversationapp.ConversationDetail{}, conversationapp.ErrForbidden
	}
	studentID, teacherID := creatorID, targetID
	if creatorRole != user.RoleStudent {
		studentID, teacherID = targetID, creatorID
	}
	var permitted bool
	targetRole := user.RoleStudent
	if creatorRole == user.RoleStudent {
		targetRole = user.RoleTeacher
	}
	if err := r.DB().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM public.users u WHERE u.id = $1 AND u.role::text = $2
		)`, targetID, targetRole.DBValue()).Scan(&permitted); err != nil {
		return conversationapp.ConversationDetail{}, err
	}
	if !permitted {
		return conversationapp.ConversationDetail{}, conversationapp.ErrForbidden
	}
	convID, err := newUUID()
	if err != nil {
		return conversationapp.ConversationDetail{}, err
	}

	tx, err := r.beginTx(ctx)
	if err != nil {
		return conversationapp.ConversationDetail{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO public.conversations (id, student_id, teacher_id, subject, last_message_at)
		VALUES ($1, $2, $3, $4, $5)`,
		convID, studentID, teacherID, subject, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return conversationapp.ConversationDetail{}, conversationapp.ErrConflict
		}
		return conversationapp.ConversationDetail{}, err
	}

	if strings.TrimSpace(initialMessage) != "" {
		msgID, err := newUUID()
		if err != nil {
			return conversationapp.ConversationDetail{}, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO public.conversation_messages (id, conversation_id, sender_id, sender_role, text, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			msgID, convID, creatorID, string(creatorRole), initialMessage, now,
		)
		if err != nil {
			return conversationapp.ConversationDetail{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return conversationapp.ConversationDetail{}, err
	}

	detail, found, err := r.GetConversation(ctx, convID, creatorID, 1, 50)
	if err != nil || !found {
		return conversationapp.ConversationDetail{}, err
	}
	return detail, nil
}

// SendMessage adds a message and updates last_message_at.
func (r ConversationRepository) SendMessage(ctx context.Context, conversationID string, senderID string, senderRole string, text string, now time.Time) (conversationapp.Message, error) {
	msgID, err := newUUID()
	if err != nil {
		return conversationapp.Message{}, err
	}

	tx, err := r.beginTx(ctx)
	if err != nil {
		return conversationapp.Message{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		INSERT INTO public.conversation_messages (id, conversation_id, sender_id, sender_role, text, created_at)
		SELECT $1::character varying, c.id::character varying, $3::character varying, $4::character varying, $5, $6
		FROM public.conversations c
		WHERE c.id::text = $2::text
		  AND ((c.student_id::text = $3::text AND $4 = 'student') OR (c.teacher_id::text = $3::text AND $4 = 'teacher'))`,
		msgID, conversationID, senderID, senderRole, text, now,
	)
	if err != nil {
		return conversationapp.Message{}, err
	}
	if tag.RowsAffected() == 0 {
		return conversationapp.Message{}, conversationapp.ErrNotFound
	}

	_, err = tx.Exec(ctx, `
		UPDATE public.conversations SET last_message_at = $1, updated_at = $1 WHERE id = $2`,
		now, conversationID,
	)
	if err != nil {
		return conversationapp.Message{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return conversationapp.Message{}, err
	}

	return conversationapp.Message{
		ID:   msgID,
		From: senderRole,
		Text: text,
		Time: now,
	}, nil
}

// ArchiveConversation sets is_archived = true for a student-owned conversation.
func (r ConversationRepository) ArchiveConversation(ctx context.Context, conversationID string, studentID string) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.conversations SET is_archived = true, updated_at = now()
		WHERE id = $1 AND student_id = $2`,
		conversationID, studentID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteConversation removes a conversation (student only).
func (r ConversationRepository) DeleteConversation(ctx context.Context, conversationID string, studentID string) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		DELETE FROM public.conversations WHERE id = $1 AND student_id = $2`,
		conversationID, studentID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListTeacherContacts returns the teachers that a student can message.
func (r ConversationRepository) ListTeacherContacts(ctx context.Context, studentID string) ([]conversationapp.Contact, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT DISTINCT u.id, COALESCE(u.display_name, u.username), c.name
		FROM public.users u
		JOIN public.classes c ON c.teacher_id = u.id
		JOIN public.class_enrollments ce ON ce.class_id = c.id
		WHERE ce.student_id = $1`,
		studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := make([]conversationapp.Contact, 0)
	for rows.Next() {
		var c conversationapp.Contact
		if err := rows.Scan(&c.ID, &c.TeacherName, &c.Scope); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// ListStudentContacts returns students in the teacher's classes.
func (r ConversationRepository) ListStudentContacts(ctx context.Context, teacherID string) ([]conversationapp.Contact, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT DISTINCT u.id, COALESCE(u.display_name, u.username), c.name
		FROM public.users u
		JOIN public.class_enrollments ce ON ce.student_id = u.id
		JOIN public.classes c ON c.id = ce.class_id
		WHERE c.teacher_id = $1`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := make([]conversationapp.Contact, 0)
	for rows.Next() {
		var c conversationapp.Contact
		if err := rows.Scan(&c.ID, &c.TeacherName, &c.Scope); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// SearchContacts searches all users by ID or display name, filtered by role.
func (r ConversationRepository) SearchContacts(ctx context.Context, query string, role user.Role) ([]conversationapp.Contact, error) {
	targetRole := "TEACHER"
	if role == user.RoleTeacher {
		targetRole = "STUDENT"
	}
	rows, err := r.DB().Query(ctx, `
		SELECT u.id, COALESCE(u.display_name, u.username), '' AS scope
		FROM public.users u
		WHERE u.role::text = $1
		  AND (u.id ILIKE $2 OR u.display_name ILIKE $2 OR u.username ILIKE $2)
		ORDER BY u.display_name
		LIMIT 20`, targetRole, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := make([]conversationapp.Contact, 0)
	for rows.Next() {
		var c conversationapp.Contact
		if err := rows.Scan(&c.ID, &c.TeacherName, &c.Scope); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

func (r ConversationRepository) beginTx(ctx context.Context) (pgx.Tx, error) {
	if r.beginner == nil {
		return nil, conversationapp.ErrConflict
	}
	return r.beginner.BeginTx(ctx, pgx.TxOptions{})
}

func pgIdx(n int) string {
	return strconv.Itoa(n)
}

func idxStr(n int) string {
	return strconv.Itoa(n)
}
