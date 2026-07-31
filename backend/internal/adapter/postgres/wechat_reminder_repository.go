package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	wechatreminder "mathstudy/backend/internal/application/wechatreminder"
)

const (
	maxWechatReminderAppIDBytes = 64
	maxWechatReminderIDBytes    = 36
	maxWechatReminderErrorBytes = 64
	maxWechatReminderBatchSize  = 100
	maxWechatReminderCleanup    = 1000
)

// WechatReminderEnqueuer atomically adds reminder jobs through a caller-owned transaction.
// Its zero value is disabled and is safe to embed in repositories.
type WechatReminderEnqueuer struct {
	enabled bool
	appID   string
}

// NewWechatReminderEnqueuer creates a content-free reminder job writer.
func NewWechatReminderEnqueuer(enabled bool, appID string) (WechatReminderEnqueuer, error) {
	appID = strings.TrimSpace(appID)
	if !enabled {
		return WechatReminderEnqueuer{}, nil
	}
	if !validWechatReminderText(appID, maxWechatReminderAppIDBytes) {
		return WechatReminderEnqueuer{}, errors.New("invalid wechat reminder app ID")
	}
	return WechatReminderEnqueuer{enabled: true, appID: appID}, nil
}

// Enabled reports whether durable reminder writes are configured.
func (e WechatReminderEnqueuer) Enabled() bool {
	return e.enabled
}

// Enqueue inserts one semantic reminder event and ignores an exact duplicate.
func (e WechatReminderEnqueuer) Enqueue(
	ctx context.Context,
	db Querier,
	eventType wechatreminder.EventType,
	sourceID string,
	recipientUserID string,
) error {
	if !e.enabled {
		return nil
	}
	if db == nil || !validWechatReminderEvent(eventType) ||
		!validWechatReminderText(sourceID, maxWechatReminderIDBytes) ||
		!validWechatReminderText(recipientUserID, maxWechatReminderIDBytes) {
		return errors.New("invalid wechat reminder enqueue input")
	}
	_, err := db.Exec(ctx, `
		INSERT INTO public.wechat_message_reminder_jobs (
			app_id, event_type, source_id, recipient_user_id
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (app_id, event_type, source_id, recipient_user_id) DO NOTHING`,
		e.appID,
		string(eventType),
		sourceID,
		recipientUserID,
	)
	if err != nil {
		return fmt.Errorf("enqueue wechat reminder: %w", err)
	}
	return nil
}

// EnqueueNoticeRecipients inserts one reminder per immutable notice recipient snapshot.
func (e WechatReminderEnqueuer) EnqueueNoticeRecipients(ctx context.Context, db Querier, noticeID string) error {
	if !e.enabled {
		return nil
	}
	if db == nil || !validWechatReminderText(noticeID, maxWechatReminderIDBytes) {
		return errors.New("invalid wechat notice reminder enqueue input")
	}
	_, err := db.Exec(ctx, `
		INSERT INTO public.wechat_message_reminder_jobs (
			app_id, event_type, source_id, recipient_user_id
		)
		SELECT $1, $2, nr.notice_id, nr.student_id
		FROM public.notice_recipients nr
		WHERE nr.notice_id = $3
		ON CONFLICT (app_id, event_type, source_id, recipient_user_id) DO NOTHING`,
		e.appID,
		string(wechatreminder.EventNotice),
		noticeID,
	)
	if err != nil {
		return fmt.Errorf("enqueue wechat notice reminders: %w", err)
	}
	return nil
}

// EnqueueDailyQuestionRecipients creates one WeChat job for each current class
// member whose daily question is ready and still unfinished. The caller owns the
// transaction that creates the source event, so the event and all jobs commit together.
func (e WechatReminderEnqueuer) EnqueueDailyQuestionRecipients(
	ctx context.Context,
	db Querier,
	eventID string,
	classID string,
	assignmentDate time.Time,
) (int, error) {
	if !e.enabled {
		return 0, nil
	}
	if db == nil || !validWechatReminderText(eventID, maxWechatReminderIDBytes) ||
		!validWechatReminderText(classID, maxWechatReminderIDBytes) || assignmentDate.IsZero() {
		return 0, errors.New("invalid daily question wechat reminder enqueue input")
	}
	tag, err := db.Exec(ctx, `
		WITH recipients AS (
			SELECT DISTINCT assignment.student_id
			FROM public.daily_question_assignments assignment
			JOIN public.class_enrollments enrollment
			  ON enrollment.class_id = assignment.class_id
			 AND enrollment.student_id = assignment.student_id
			JOIN public.users student
			  ON student.id = assignment.student_id
			WHERE assignment.class_id = $4
			  AND assignment.assignment_date = $5::date
			  AND assignment.status = 'ready'
			  AND assignment.content_id IS NOT NULL
			  AND student.role = 'STUDENT'::public.userrole
			  AND student.is_active = true
			  AND student.status = 'ACTIVE'::public.userstatus
		)
		INSERT INTO public.wechat_message_reminder_jobs (
			app_id, event_type, source_id, recipient_user_id
		)
		SELECT $1, $2, $3, recipients.student_id
		FROM recipients
		ON CONFLICT (app_id, event_type, source_id, recipient_user_id) DO NOTHING`,
		e.appID,
		string(wechatreminder.EventDailyQuestion),
		eventID,
		classID,
		assignmentDate,
	)
	if err != nil {
		return 0, fmt.Errorf("enqueue daily question wechat reminders: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ReconcileAutomaticDailyQuestionRecipients adds newly eligible students and
// recovers skipped or dead jobs without sending an automatic reminder twice.
func (e WechatReminderEnqueuer) ReconcileAutomaticDailyQuestionRecipients(
	ctx context.Context,
	db Querier,
	eventID string,
	classID string,
	assignmentDate time.Time,
) (int, error) {
	return e.reconcileDailyQuestionRecipients(ctx, db, eventID, classID, assignmentDate)
}

func (e WechatReminderEnqueuer) reconcileDailyQuestionRecipients(
	ctx context.Context,
	db Querier,
	eventID string,
	classID string,
	assignmentDate time.Time,
) (int, error) {
	if !e.enabled {
		return 0, nil
	}
	if db == nil || !validWechatReminderText(eventID, maxWechatReminderIDBytes) ||
		!validWechatReminderText(classID, maxWechatReminderIDBytes) || assignmentDate.IsZero() {
		return 0, errors.New("invalid daily question wechat reminder requeue input")
	}
	var queued int
	err := db.QueryRow(ctx, `
		WITH recipients AS MATERIALIZED (
			SELECT DISTINCT assignment.student_id
			FROM public.daily_question_assignments assignment
			JOIN public.class_enrollments enrollment
			  ON enrollment.class_id = assignment.class_id
			 AND enrollment.student_id = assignment.student_id
			JOIN public.users student
			  ON student.id = assignment.student_id
			WHERE assignment.class_id = $4
			  AND assignment.assignment_date = $5::date
			  AND assignment.status = 'ready'
			  AND assignment.content_id IS NOT NULL
			  AND student.role = 'STUDENT'::public.userrole
			  AND student.is_active = true
			  AND student.status = 'ACTIVE'::public.userstatus
		), reset_jobs AS (
			UPDATE public.wechat_message_reminder_jobs job
			SET status = 'pending',
				attempt_count = 0,
				next_attempt_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
				lease_owner = NULL,
				lease_expires_at = NULL,
				last_error_code = NULL,
				provider_error_code = NULL,
				finished_at = NULL
			FROM recipients
			WHERE job.app_id = $1
			  AND job.event_type = $2
			  AND job.source_id = $3
			  AND job.recipient_user_id = recipients.student_id
				  AND job.status IN ('skipped', 'dead')
			RETURNING job.id
		), inserted_jobs AS (
			INSERT INTO public.wechat_message_reminder_jobs (
				app_id, event_type, source_id, recipient_user_id
			)
			SELECT $1, $2, $3, recipients.student_id
			FROM recipients
			ON CONFLICT (app_id, event_type, source_id, recipient_user_id) DO NOTHING
			RETURNING id
		)
		SELECT (SELECT COUNT(*) FROM reset_jobs) + (SELECT COUNT(*) FROM inserted_jobs)`,
		e.appID,
		string(wechatreminder.EventDailyQuestion),
		eventID,
		classID,
		assignmentDate,
	).Scan(&queued)
	if err != nil {
		return 0, fmt.Errorf("reconcile daily question wechat reminders: %w", err)
	}
	return queued, nil
}

// RequeueDailyQuestionRecipient recovers one skipped or dead daily-question
// job while preserving pending, processing, and already sent deliveries.
func (e WechatReminderEnqueuer) RequeueDailyQuestionRecipient(
	ctx context.Context,
	db Querier,
	eventID string,
	recipientUserID string,
) (int, error) {
	if !e.enabled {
		return 0, nil
	}
	if db == nil || !validWechatReminderText(eventID, maxWechatReminderIDBytes) ||
		!validWechatReminderText(recipientUserID, maxWechatReminderIDBytes) {
		return 0, errors.New("invalid daily question wechat reminder recipient requeue input")
	}
	tag, err := db.Exec(ctx, `
		UPDATE public.wechat_message_reminder_jobs
		SET status = 'pending',
			attempt_count = 0,
			next_attempt_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
			lease_owner = NULL,
			lease_expires_at = NULL,
			last_error_code = NULL,
			provider_error_code = NULL,
			finished_at = NULL
		WHERE app_id = $1
		  AND event_type = $2
		  AND source_id = $3
		  AND recipient_user_id = $4
		  AND status IN ('skipped', 'dead')`,
		e.appID,
		string(wechatreminder.EventDailyQuestion),
		eventID,
		recipientUserID,
	)
	if err != nil {
		return 0, fmt.Errorf("requeue daily question wechat reminder recipient: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RequeueUnconfirmedNoticeRecipients schedules another notice reminder without duplicating active jobs.
func (e WechatReminderEnqueuer) RequeueUnconfirmedNoticeRecipients(ctx context.Context, db Querier, noticeID string) (int, error) {
	if !e.enabled {
		return 0, nil
	}
	if db == nil || !validWechatReminderText(noticeID, maxWechatReminderIDBytes) {
		return 0, errors.New("invalid wechat notice reminder requeue input")
	}
	var queued int
	err := db.QueryRow(ctx, `
		WITH target AS MATERIALIZED (
			SELECT nr.notice_id, nr.student_id
			FROM public.notice_recipients nr
			WHERE nr.notice_id = $3
			  AND NOT EXISTS (
				SELECT 1
				FROM public.notice_confirmations nc
				WHERE nc.notice_id = nr.notice_id AND nc.student_id = nr.student_id
			  )
		), reset_jobs AS (
			UPDATE public.wechat_message_reminder_jobs job
			SET status = 'pending',
				attempt_count = 0,
				next_attempt_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
				lease_owner = NULL,
				lease_expires_at = NULL,
				last_error_code = NULL,
				provider_error_code = NULL,
				finished_at = NULL
			FROM target
			WHERE job.app_id = $1
			  AND job.event_type = $2
			  AND job.source_id = target.notice_id
			  AND job.recipient_user_id = target.student_id
			  AND job.status IN ('sent', 'skipped', 'dead')
			RETURNING job.id
		), inserted_jobs AS (
			INSERT INTO public.wechat_message_reminder_jobs (
				app_id, event_type, source_id, recipient_user_id
			)
			SELECT $1, $2, target.notice_id, target.student_id
			FROM target
			ON CONFLICT (app_id, event_type, source_id, recipient_user_id) DO NOTHING
			RETURNING id
		)
		SELECT (SELECT COUNT(*) FROM reset_jobs) + (SELECT COUNT(*) FROM inserted_jobs)`,
		e.appID,
		string(wechatreminder.EventNotice),
		noticeID,
	).Scan(&queued)
	if err != nil {
		return 0, fmt.Errorf("requeue unconfirmed notice reminders: %w", err)
	}
	return queued, nil
}

// WechatReminderRepository owns durable job leases and delivery eligibility checks.
type WechatReminderRepository struct {
	Repository
}

// NewWechatReminderRepository creates a PostgreSQL-backed reminder repository.
func NewWechatReminderRepository(db Querier) (WechatReminderRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return WechatReminderRepository{}, err
	}
	return WechatReminderRepository{Repository: base}, nil
}

// Claim leases due pending jobs and expired processing jobs without blocking other workers.
func (r WechatReminderRepository) Claim(
	ctx context.Context,
	appID string,
	owner string,
	now time.Time,
	leaseExpiresAt time.Time,
	batchSize int,
) ([]wechatreminder.Job, error) {
	if !validWechatReminderText(appID, maxWechatReminderAppIDBytes) ||
		!validWechatReminderText(owner, maxWechatReminderIDBytes) ||
		now.IsZero() || !leaseExpiresAt.After(now) ||
		batchSize <= 0 || batchSize > maxWechatReminderBatchSize {
		return nil, errors.New("invalid wechat reminder claim input")
	}
	rows, err := r.DB().Query(ctx, `
		WITH candidates AS (
			SELECT job.id
			FROM public.wechat_message_reminder_jobs job
			WHERE job.app_id = $1
			  AND (
				(job.status = 'pending' AND job.next_attempt_at <= $3)
				OR (job.status = 'processing' AND job.lease_expires_at <= $3)
			  )
			ORDER BY
				CASE
					WHEN job.status = 'pending' THEN job.next_attempt_at
					ELSE job.lease_expires_at
				END,
				job.created_at,
				job.id
			FOR UPDATE SKIP LOCKED
			LIMIT $5
		), leased AS (
			UPDATE public.wechat_message_reminder_jobs job
			SET status = 'processing',
				attempt_count = job.attempt_count + 1,
				lease_owner = $2,
				lease_expires_at = $4,
				finished_at = NULL
			FROM candidates
			WHERE job.id = candidates.id
			RETURNING job.id, job.event_type, job.source_id,
				job.recipient_user_id, job.attempt_count
		)
		SELECT id, event_type, source_id, recipient_user_id, attempt_count
		FROM leased
		ORDER BY id`,
		appID,
		owner,
		now,
		leaseExpiresAt,
		batchSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]wechatreminder.Job, 0)
	for rows.Next() {
		var job wechatreminder.Job
		var eventType string
		if err := rows.Scan(
			&job.ID,
			&eventType,
			&job.SourceID,
			&job.RecipientUserID,
			&job.AttemptCount,
		); err != nil {
			return nil, err
		}
		job.EventType = wechatreminder.EventType(eventType)
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// ResolveDelivery checks source state and current binding state immediately before a send.
func (r WechatReminderRepository) ResolveDelivery(
	ctx context.Context,
	appID string,
	job wechatreminder.Job,
) (wechatreminder.Delivery, bool, string, error) {
	delivery, actionable, err := r.resolveSourceDelivery(ctx, job)
	if err != nil {
		return wechatreminder.Delivery{}, false, "", err
	}
	if !actionable {
		return wechatreminder.Delivery{}, false, "source_not_actionable", nil
	}

	var openID string
	err = r.DB().QueryRow(ctx, `
		SELECT binding.open_id
		FROM public.users recipient
		JOIN public.wechat_user_bindings binding
		  ON binding.user_id = recipient.id
		 AND binding.app_id = $1
		WHERE recipient.id = $2
		  AND recipient.is_active = true
		  AND recipient.status = 'ACTIVE'::public.userstatus
		  AND binding.subscribed = true
		  AND binding.bound_at IS NOT NULL`,
		appID,
		job.RecipientUserID,
	).Scan(&openID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return wechatreminder.Delivery{}, false, "recipient_unavailable", nil
		}
		return wechatreminder.Delivery{}, false, "", err
	}
	delivery.OpenID = openID
	return delivery, true, "", nil
}

func (r WechatReminderRepository) resolveSourceDelivery(
	ctx context.Context,
	job wechatreminder.Job,
) (wechatreminder.Delivery, bool, error) {
	var query string
	switch job.EventType {
	case wechatreminder.EventPrivateMessage:
		query = `
			SELECT COALESCE(NULLIF(BTRIM(sender.display_name), ''), sender.username),
				CASE WHEN message.text <> '' THEN message.text ELSE '[附件]' END,
				message.created_at AT TIME ZONE 'Asia/Shanghai'
			FROM public.conversation_messages message
			JOIN public.conversations conversation
			  ON conversation.id = message.conversation_id
			JOIN public.users sender
			  ON sender.id = message.sender_id
			WHERE message.id = $1
			  AND message.sender_id <> $2
			  AND message.read_at IS NULL
			  AND (
				(message.sender_role = 'student'
				  AND message.sender_id = conversation.student_id
				  AND conversation.teacher_id = $2)
				OR (message.sender_role = 'teacher'
				  AND message.sender_id = conversation.teacher_id
				  AND conversation.student_id = $2)
			  )`
	case wechatreminder.EventNotice:
		query = `
			SELECT COALESCE(
					NULLIF(BTRIM(publisher.display_name), ''),
					NULLIF(BTRIM(publisher.username), ''),
					'老师'
				),
				notice.title,
				notice.created_at AT TIME ZONE 'Asia/Shanghai'
			FROM public.notice_recipients recipient
			JOIN public.notices notice
			  ON notice.id = recipient.notice_id
			LEFT JOIN public.users publisher
			  ON publisher.id = notice.teacher_id
			WHERE recipient.notice_id = $1
			  AND recipient.student_id = $2
			  AND NOT EXISTS (
				SELECT 1
				FROM public.notice_confirmations confirmation
				WHERE confirmation.notice_id = recipient.notice_id
				  AND confirmation.student_id = recipient.student_id
			  )`
	case wechatreminder.EventQAMessage:
		query = `
			SELECT COALESCE(NULLIF(BTRIM(sender.display_name), ''), sender.username),
				CASE WHEN message.text <> '' THEN message.text ELSE '[附件]' END,
				message.created_at AT TIME ZONE 'Asia/Shanghai'
			FROM public.question_thread_messages message
			JOIN public.question_threads thread
			  ON thread.id = message.thread_id
			JOIN public.users sender
			  ON sender.id = message.sender_id
			WHERE message.id = $1
			  AND message.sender_id <> $2
			  AND message.read_at IS NULL
			  AND (
				(message.sender_role = 'student'
				  AND message.sender_id = thread.student_id
				  AND thread.teacher_id = $2)
				OR (message.sender_role = 'teacher'
				  AND message.sender_id = thread.teacher_id
				  AND thread.student_id = $2)
			  )`
	case wechatreminder.EventDailyQuestion:
		query = `
			SELECT CASE
			           WHEN event.kind = 'uniform_low_stock' THEN '每日一题'
			           ELSE COALESCE(
			               NULLIF(BTRIM(teacher.display_name), ''),
			               NULLIF(BTRIM(teacher.username), ''),
			               '老师'
			           )
			       END,
			       CASE
			           WHEN event.kind = 'uniform_low_stock'
			           THEN '班级统一题日程只剩 1 道题，请及时补充。'
			           ELSE '今日每日一题尚未完成，请及时作答。'
			       END,
			       event.created_at AT TIME ZONE 'Asia/Shanghai'
			FROM public.daily_question_wechat_events event
			JOIN public.classes class
			  ON class.id = event.class_id
			LEFT JOIN public.users teacher
			  ON teacher.id = event.teacher_id
			LEFT JOIN public.daily_question_class_settings settings
			  ON settings.class_id = event.class_id
			WHERE event.id = $1
			  AND class.teacher_id = event.teacher_id
			  AND (
			      (
			          event.kind IN ('manual_student_reminder', 'automatic_student_reminder')
			          AND (event.kind <> 'automatic_student_reminder'
			               OR coalesce(settings.auto_reminder_enabled, false))
			          AND EXISTS (
			              SELECT 1
			              FROM public.daily_question_assignments assignment
			              JOIN public.class_enrollments enrollment
			                ON enrollment.class_id = assignment.class_id
			               AND enrollment.student_id = assignment.student_id
			              WHERE assignment.class_id = event.class_id
			                AND assignment.assignment_date = event.assignment_date
			                AND event.assignment_date = (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date
			                AND assignment.student_id = $2
			                AND assignment.status = 'ready'
			                AND assignment.content_id IS NOT NULL
			          )
			      )
			      OR (
			          event.kind = 'uniform_low_stock'
			          AND event.teacher_id = $2
			          AND settings.strategy = 'uniform'
			          AND EXISTS (
			              SELECT 1
			              FROM public.daily_question_class_selections selection
			              JOIN public.contents content ON content.id = selection.content_id
			              WHERE selection.class_id = event.class_id
			                AND selection.content_id = event.remaining_content_id
			                AND selection.assignment_date >= (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date
			                AND selection.source = 'teacher_bank'
			                AND selection.selection_reason = 'teacher_uniform'
			                AND content.type = 'PROBLEM'::public.contenttype
			                AND content.status = 'PUBLISHED'::public.contentstatus
			                AND content.deleted_at IS NULL
			                AND NOT EXISTS (
			                    SELECT 1
			                    FROM public.daily_question_assignments assignment
			                    WHERE assignment.class_id = selection.class_id
			                      AND assignment.assignment_date = selection.assignment_date
			                      AND (assignment.status <> 'unavailable' OR assignment.content_id IS NOT NULL)
			                )
			          )
			          AND 1 = (
			              SELECT count(*)
			              FROM public.daily_question_class_selections selection
			              JOIN public.contents content ON content.id = selection.content_id
			              WHERE selection.class_id = event.class_id
			                AND selection.assignment_date >= (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date
			                AND selection.source = 'teacher_bank'
			                AND selection.selection_reason = 'teacher_uniform'
			                AND content.type = 'PROBLEM'::public.contenttype
			                AND content.status = 'PUBLISHED'::public.contentstatus
			                AND content.deleted_at IS NULL
			                AND NOT EXISTS (
			                    SELECT 1
			                    FROM public.daily_question_assignments assignment
			                    WHERE assignment.class_id = selection.class_id
			                      AND assignment.assignment_date = selection.assignment_date
			                      AND (assignment.status <> 'unavailable' OR assignment.content_id IS NOT NULL)
			                )
			          )
			      )
			  )`
	default:
		return wechatreminder.Delivery{}, false, nil
	}
	var delivery wechatreminder.Delivery
	if err := r.DB().QueryRow(ctx, query, job.SourceID, job.RecipientUserID).Scan(
		&delivery.ActorName,
		&delivery.Content,
		&delivery.OccurredAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return wechatreminder.Delivery{}, false, nil
		}
		return wechatreminder.Delivery{}, false, err
	}
	return delivery, true, nil
}

// RenewLease confirms ownership and extends a live lease immediately before sending.
func (r WechatReminderRepository) RenewLease(
	ctx context.Context,
	id int64,
	owner string,
	now time.Time,
	leaseExpiresAt time.Time,
) (bool, error) {
	if id <= 0 || !validWechatReminderText(owner, maxWechatReminderIDBytes) ||
		now.IsZero() || !leaseExpiresAt.After(now) {
		return false, errors.New("invalid wechat reminder lease renewal")
	}
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.wechat_message_reminder_jobs
		SET lease_expires_at = $4
		WHERE id = $1
		  AND status = 'processing'
		  AND lease_owner = $2
		  AND lease_expires_at > $3`,
		id,
		owner,
		now,
		leaseExpiresAt,
	)
	return tag.RowsAffected() == 1, err
}

// MarkSent completes a job only while the caller still owns its lease.
func (r WechatReminderRepository) MarkSent(ctx context.Context, id int64, owner string, finishedAt time.Time) (bool, error) {
	if !validWechatReminderTransition(id, owner, finishedAt) {
		return false, errors.New("invalid wechat reminder sent transition")
	}
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.wechat_message_reminder_jobs
		SET status = 'sent',
			lease_owner = NULL,
			lease_expires_at = NULL,
			last_error_code = NULL,
			provider_error_code = NULL,
			finished_at = $3
		WHERE id = $1 AND status = 'processing' AND lease_owner = $2`,
		id,
		owner,
		finishedAt,
	)
	return tag.RowsAffected() == 1, err
}

// MarkSkipped completes an ineligible or provider-rejected recipient job.
func (r WechatReminderRepository) MarkSkipped(
	ctx context.Context,
	id int64,
	owner string,
	errorCode string,
	providerCode *int,
	finishedAt time.Time,
) (bool, error) {
	if !validWechatReminderTransition(id, owner, finishedAt) ||
		!validWechatReminderText(errorCode, maxWechatReminderErrorBytes) {
		return false, errors.New("invalid wechat reminder skip code")
	}
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.wechat_message_reminder_jobs
		SET status = 'skipped',
			lease_owner = NULL,
			lease_expires_at = NULL,
			last_error_code = $3,
			provider_error_code = $4,
			finished_at = $5
		WHERE id = $1 AND status = 'processing' AND lease_owner = $2`,
		id,
		owner,
		errorCode,
		providerCode,
		finishedAt,
	)
	return tag.RowsAffected() == 1, err
}

// Reschedule releases a lease and makes a transient failure eligible later.
func (r WechatReminderRepository) Reschedule(
	ctx context.Context,
	id int64,
	owner string,
	errorCode string,
	providerCode *int,
	nextAttemptAt time.Time,
) (bool, error) {
	if id <= 0 || !validWechatReminderText(owner, maxWechatReminderIDBytes) ||
		nextAttemptAt.IsZero() || !validWechatReminderText(errorCode, maxWechatReminderErrorBytes) {
		return false, errors.New("invalid wechat reminder retry code")
	}
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.wechat_message_reminder_jobs
		SET status = 'pending',
			next_attempt_at = $5,
			lease_owner = NULL,
			lease_expires_at = NULL,
			last_error_code = $3,
			provider_error_code = $4,
			finished_at = NULL
		WHERE id = $1 AND status = 'processing' AND lease_owner = $2`,
		id,
		owner,
		errorCode,
		providerCode,
		nextAttemptAt,
	)
	return tag.RowsAffected() == 1, err
}

// MarkDead completes a permanently failed or exhausted job.
func (r WechatReminderRepository) MarkDead(
	ctx context.Context,
	id int64,
	owner string,
	errorCode string,
	providerCode *int,
	finishedAt time.Time,
) (bool, error) {
	if !validWechatReminderTransition(id, owner, finishedAt) ||
		!validWechatReminderText(errorCode, maxWechatReminderErrorBytes) {
		return false, errors.New("invalid wechat reminder dead-letter code")
	}
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.wechat_message_reminder_jobs
		SET status = 'dead',
			lease_owner = NULL,
			lease_expires_at = NULL,
			last_error_code = $3,
			provider_error_code = $4,
			finished_at = $5
		WHERE id = $1 AND status = 'processing' AND lease_owner = $2`,
		id,
		owner,
		errorCode,
		providerCode,
		finishedAt,
	)
	return tag.RowsAffected() == 1, err
}

// DeleteFinishedBefore removes a bounded batch of expired terminal jobs.
func (r WechatReminderRepository) DeleteFinishedBefore(
	ctx context.Context,
	appID string,
	before time.Time,
	batchSize int,
) (int64, error) {
	if !validWechatReminderText(appID, maxWechatReminderAppIDBytes) || before.IsZero() ||
		batchSize <= 0 || batchSize > maxWechatReminderCleanup {
		return 0, errors.New("invalid wechat reminder cleanup input")
	}
	tag, err := r.DB().Exec(ctx, `
		WITH expired AS (
			SELECT job.id
			FROM public.wechat_message_reminder_jobs job
			WHERE job.app_id = $1
			  AND job.status IN ('sent', 'skipped', 'dead')
			  AND job.finished_at < $2
			ORDER BY job.finished_at, job.id
			FOR UPDATE SKIP LOCKED
			LIMIT $3
		)
		DELETE FROM public.wechat_message_reminder_jobs job
		USING expired
		WHERE job.id = expired.id`,
		appID,
		before,
		batchSize,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func validWechatReminderEvent(eventType wechatreminder.EventType) bool {
	switch eventType {
	case wechatreminder.EventPrivateMessage, wechatreminder.EventNotice, wechatreminder.EventQAMessage, wechatreminder.EventDailyQuestion:
		return true
	default:
		return false
	}
}

func validWechatReminderText(value string, maxBytes int) bool {
	return value != "" && len(value) <= maxBytes && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validWechatReminderTransition(id int64, owner string, finishedAt time.Time) bool {
	return id > 0 && validWechatReminderText(owner, maxWechatReminderIDBytes) && !finishedAt.IsZero()
}
