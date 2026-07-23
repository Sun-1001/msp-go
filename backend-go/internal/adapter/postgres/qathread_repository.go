package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	qathreadapp "mathstudy/backend-go/internal/application/qathread"
	"mathstudy/backend-go/internal/domain/user"
)

// QAThreadRepository persists Q&A thread data in PostgreSQL.
type QAThreadRepository struct {
	Repository
}

// NewQAThreadRepository creates a PostgreSQL-backed Q&A thread repository.
func NewQAThreadRepository(db Querier) (QAThreadRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return QAThreadRepository{}, err
	}
	return QAThreadRepository{Repository: base}, nil
}

// ListThreads returns paginated threads for a user.
func (r QAThreadRepository) ListThreads(ctx context.Context, userID string, role user.Role, search string, status string, className string, teacherID string, page int, pageSize int) ([]any, int, error) {
	pgPage, err := NewPage((page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if role == user.RoleStudent {
		return r.listStudentThreads(ctx, userID, search, teacherID, pgPage)
	}
	return r.listTeacherThreads(ctx, userID, search, status, className, pgPage)
}

func (r QAThreadRepository) listStudentThreads(ctx context.Context, studentID string, search string, teacherID string, page Page) ([]any, int, error) {
	where := " WHERE qt.student_id = $1"
	args := []any{studentID}
	idx := 2
	if strings.TrimSpace(search) != "" {
		where += ` AND (qt.title ILIKE $` + idxStr(idx) + ` OR qt.context ILIKE $` + idxStr(idx) + `)`
		args = append(args, "%"+search+"%")
		idx++
	}
	if strings.TrimSpace(teacherID) != "" {
		where += ` AND qt.teacher_id = $` + idxStr(idx)
		args = append(args, teacherID)
		idx++
	}

	var total int
	if err := r.DB().QueryRow(ctx, `SELECT COUNT(*) FROM public.question_threads qt`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	countArgs := len(args)
	args = append(args, page.Limit, page.Offset)

	rows, err := r.DB().Query(ctx, `
		SELECT qt.id, qt.title, qt.teacher_id,
			COALESCE(u.display_name, u.username),
			qt.source, qt.context, qt.status, qt.updated_at
		FROM public.question_threads qt
		JOIN public.users u ON u.id = qt.teacher_id
		`+where+`
		ORDER BY qt.updated_at DESC
		LIMIT $`+idxStr(countArgs+1)+` OFFSET $`+idxStr(countArgs+2),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]any, 0)
	for rows.Next() {
		var item qathreadapp.StudentThreadItem
		if err := rows.Scan(&item.ID, &item.Title, &item.TeacherID, &item.TeacherName, &item.Source, &item.Context, &item.Status, &item.LastUpdate); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r QAThreadRepository) listTeacherThreads(ctx context.Context, teacherID string, search string, status string, className string, page Page) ([]any, int, error) {
	where := " WHERE qt.teacher_id = $1"
	args := []any{teacherID}
	idx := 2
	if strings.TrimSpace(search) != "" {
		where += ` AND (qt.title ILIKE $` + idxStr(idx) + ` OR qt.context ILIKE $` + idxStr(idx) + ` OR u.display_name ILIKE $` + idxStr(idx) + `)`
		args = append(args, "%"+search+"%")
		idx++
	}
	if strings.TrimSpace(status) != "" && status != "全部" {
		where += ` AND qt.status = $` + idxStr(idx)
		args = append(args, status)
		idx++
	}
	if strings.TrimSpace(className) != "" {
		where += ` AND qt.class_name ILIKE $` + idxStr(idx)
		args = append(args, "%"+className+"%")
		idx++
	}

	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM public.question_threads qt
		JOIN public.users u ON u.id = qt.student_id
		`+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	countArgs := len(args)
	args = append(args, page.Limit, page.Offset)

	rows, err := r.DB().Query(ctx, `
		SELECT qt.id,
			COALESCE(u.display_name, u.username),
			COALESCE(qt.class_name, ''),
			qt.title, qt.source,
			COALESCE(qt.knowledge_point, ''),
			COALESCE(qt.resource_name, ''),
			qt.status, qt.context, qt.updated_at
		FROM public.question_threads qt
		JOIN public.users u ON u.id = qt.student_id
		`+where+`
		ORDER BY qt.updated_at DESC
		LIMIT $`+idxStr(countArgs+1)+` OFFSET $`+idxStr(countArgs+2),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]any, 0)
	for rows.Next() {
		var item qathreadapp.TeacherThreadItem
		var resourceName, knowledgePoint pgtype.Text
		if err := rows.Scan(&item.ID, &item.StudentName, &item.ClassName, &item.Title, &item.Source,
			&knowledgePoint, &resourceName, &item.Status, &item.Context, &item.LastUpdate); err != nil {
			return nil, 0, err
		}
		if knowledgePoint.Valid {
			item.KnowledgePoint = knowledgePoint.String
		}
		if resourceName.Valid {
			item.ResourceName = resourceName.String
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// GetThread returns a thread with full message history.
func (r QAThreadRepository) GetThread(ctx context.Context, threadID string, userID string, role user.Role, page, pageSize int) (any, bool, error) {
	if role == user.RoleStudent {
		return r.getStudentThread(ctx, threadID, userID, page, pageSize)
	}
	return r.getTeacherThread(ctx, threadID, userID, page, pageSize)
}

func (r QAThreadRepository) getStudentThread(ctx context.Context, threadID string, studentID string, page, pageSize int) (any, bool, error) {
	var detail qathreadapp.ThreadDetail
	err := r.DB().QueryRow(ctx, `
		SELECT qt.id, qt.title, qt.teacher_id,
			COALESCE(u.display_name, u.username),
			qt.source, qt.context, qt.status
		FROM public.question_threads qt
		JOIN public.users u ON u.id = qt.teacher_id
		WHERE qt.id = $1 AND qt.student_id = $2`,
		threadID, studentID,
	).Scan(&detail.ID, &detail.Title, &detail.TeacherID, &detail.TeacherName, &detail.Source, &detail.Context, &detail.Status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	msgs, total, err := r.loadThreadMessages(ctx, threadID, page, pageSize)
	if err != nil {
		return nil, false, err
	}
	detail.Messages = msgs
	detail.MessagesTotal, detail.MessagesPage, detail.MessagesSize = total, page, pageSize
	return detail, true, nil
}

func (r QAThreadRepository) getTeacherThread(ctx context.Context, threadID string, teacherID string, page, pageSize int) (any, bool, error) {
	var detail qathreadapp.ThreadDetail
	var knowledgePoint, resourceName pgtype.Text
	err := r.DB().QueryRow(ctx, `
		SELECT qt.id,
			COALESCE(u.display_name, u.username),
			qt.title, qt.source,
			COALESCE(qt.knowledge_point, ''),
			COALESCE(qt.resource_name, ''),
			qt.status, qt.context
		FROM public.question_threads qt
		JOIN public.users u ON u.id = qt.student_id
		WHERE qt.id = $1 AND qt.teacher_id = $2`,
		threadID, teacherID,
	).Scan(&detail.ID, &detail.StudentName, &detail.Title, &detail.Source,
		&knowledgePoint, &resourceName, &detail.Status, &detail.Context)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if knowledgePoint.Valid {
		detail.KnowledgePoint = knowledgePoint.String
	}
	if resourceName.Valid {
		detail.ResourceName = resourceName.String
	}
	msgs, total, err := r.loadThreadMessages(ctx, threadID, page, pageSize)
	if err != nil {
		return nil, false, err
	}
	detail.Messages = msgs
	detail.MessagesTotal, detail.MessagesPage, detail.MessagesSize = total, page, pageSize
	return detail, true, nil
}

func (r QAThreadRepository) loadThreadMessages(ctx context.Context, threadID string, page, pageSize int) ([]qathreadapp.Message, int, error) {
	if page < 1 { page = 1 }
	pgPage, err := NewPage((page-1)*pageSize, pageSize)
	if err != nil { return nil, 0, err }
	var total int
	if err := r.DB().QueryRow(ctx, `SELECT COUNT(*) FROM public.question_thread_messages WHERE thread_id = $1`, threadID).Scan(&total); err != nil { return nil, 0, err }
	rows, err := r.DB().Query(ctx, `
		SELECT id, sender_role, text, created_at
		FROM public.question_thread_messages
		WHERE thread_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, threadID, pgPage.Limit, pgPage.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	msgs := make([]qathreadapp.Message, 0)
	for rows.Next() {
		var m qathreadapp.Message
		if err := rows.Scan(&m.ID, &m.From, &m.Text, &m.Time); err != nil {
			return nil, 0, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil { return nil, 0, err }
	for left, right := 0, len(msgs)-1; left < right; left, right = left+1, right-1 { msgs[left], msgs[right] = msgs[right], msgs[left] }
	return msgs, total, nil
}

// extractQuestionPart returns the student's own question (before the --- separator).
func extractQuestionPart(content string) string {
	parts := strings.SplitN(content, "\n---\n", 2)
	if len(parts) == 2 {
		q := strings.TrimSpace(parts[0])
		if q != "" && !strings.HasPrefix(q, "【原题】") {
			return q
		}
	}
	return ""
}

// extractTitle returns a short title, preferring the 【原题】 line for imports.
func extractTitle(content string, maxLen int) string {
	// First pass: look for a line that starts with 【原题】
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "【原题】") {
			cleaned := strings.TrimPrefix(trimmed, "【原题】")
			cleaned = strings.TrimSpace(cleaned)
			if cleaned != "" {
				runes := []rune(cleaned)
				if len(runes) > maxLen {
					return string(runes[:maxLen]) + "..."
				}
				return cleaned
			}
		}
	}
	// Second pass: first non-empty line
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		runes := []rune(trimmed)
		if len(runes) > maxLen {
			return string(runes[:maxLen]) + "..."
		}
		return trimmed
	}
	return ""
}

// CreateThread creates a new question thread with the first message.
func (r QAThreadRepository) CreateThread(ctx context.Context, studentID string, teacherID string, content string, source string, now time.Time) (qathreadapp.ThreadDetail, error) {
	var permitted bool
	if err := r.DB().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM public.classes c
			JOIN public.class_enrollments ce ON ce.class_id = c.id
			WHERE c.teacher_id = $1 AND ce.student_id = $2
		)`, teacherID, studentID).Scan(&permitted); err != nil {
		return qathreadapp.ThreadDetail{}, err
	}
	if !permitted {
		return qathreadapp.ThreadDetail{}, qathreadapp.ErrForbidden
	}
	threadID, err := newUUID()
	if err != nil {
		return qathreadapp.ThreadDetail{}, err
	}
	title := extractTitle(content, 30)
	threadContext := content
	firstMsg := content
	if strings.Contains(content, "【原题】") {
		questionPart := extractQuestionPart(content)
		if questionPart != "" {
			firstMsg = questionPart
			// Keep only the mistake details in context (strip the question prefix)
			parts := strings.SplitN(content, "\n---\n", 2)
			if len(parts) == 2 {
				threadContext = strings.TrimSpace(parts[1])
			}
		} else {
			firstMsg = "从" + source + "导入了一道题目，请老师帮忙分析"
		}
	}

	tx, err := r.beginTx(ctx)
	if err != nil {
		return qathreadapp.ThreadDetail{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO public.question_threads (id, student_id, teacher_id, title, source, context, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, '待回复', $7, $7)`,
		threadID, studentID, teacherID, title, source, threadContext, now,
	)
	if err != nil {
		return qathreadapp.ThreadDetail{}, err
	}

	msgID, err := newUUID()
	if err != nil {
		return qathreadapp.ThreadDetail{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO public.question_thread_messages (id, thread_id, sender_id, sender_role, text, created_at)
		VALUES ($1, $2, $3, 'student', $4, $5)`,
		msgID, threadID, studentID, firstMsg, now,
	)
	if err != nil {
		return qathreadapp.ThreadDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return qathreadapp.ThreadDetail{}, err
	}

	return qathreadapp.ThreadDetail{
		ID:       threadID,
		Title:    title,
		Source:   source,
		Context:  threadContext,
		Status:   "待回复",
		Messages: []qathreadapp.Message{{ID: msgID, From: "student", Text: firstMsg, Time: now}},
	}, nil
}

// CreateThreadMessage adds a message to a thread and updates status.
func (r QAThreadRepository) CreateThreadMessage(ctx context.Context, threadID string, senderID string, senderRole string, text string, now time.Time) (qathreadapp.Message, error) {
	msgID, err := newUUID()
	if err != nil {
		return qathreadapp.Message{}, err
	}

	tx, err := r.beginTx(ctx)
	if err != nil {
		return qathreadapp.Message{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		INSERT INTO public.question_thread_messages (id, thread_id, sender_id, sender_role, text, created_at)
		SELECT $1::character varying, qt.id::character varying, $3::character varying, $4::character varying, $5, $6
		FROM public.question_threads qt
		WHERE qt.id::text = $2::text
		  AND ((qt.student_id::text = $3::text AND $4 = 'student') OR (qt.teacher_id::text = $3::text AND $4 = 'teacher'))`,
		msgID, threadID, senderID, senderRole, text, now,
	)
	if err != nil {
		return qathreadapp.Message{}, err
	}
	if tag.RowsAffected() == 0 {
		return qathreadapp.Message{}, qathreadapp.ErrNotFound
	}

	// Update status: student follow-up → 待回复, teacher reply → 已回复
	newStatus := "待回复"
	if senderRole == "teacher" {
		newStatus = "已回复"
	}
	_, err = tx.Exec(ctx, `
		UPDATE public.question_threads SET status = $1, updated_at = $2 WHERE id = $3`,
		newStatus, now, threadID,
	)
	if err != nil {
		return qathreadapp.Message{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return qathreadapp.Message{}, err
	}

	return qathreadapp.Message{ID: msgID, From: senderRole, Text: text, Time: now}, nil
}

// UpdateThreadStatus updates a thread's status (teacher only).
func (r QAThreadRepository) UpdateThreadStatus(ctx context.Context, threadID string, teacherID string, status string) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.question_threads SET status = $1, updated_at = now()
		WHERE id = $2 AND teacher_id = $3`,
		status, threadID, teacherID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteThread removes a thread and its messages (student only).
func (r QAThreadRepository) DeleteThread(ctx context.Context, threadID string, studentID string) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		DELETE FROM public.question_threads WHERE id = $1 AND student_id = $2`,
		threadID, studentID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r QAThreadRepository) beginTx(ctx context.Context) (pgx.Tx, error) {
	if r.beginner == nil {
		return nil, qathreadapp.ErrNotFound
	}
	return r.beginner.BeginTx(ctx, pgx.TxOptions{})
}
