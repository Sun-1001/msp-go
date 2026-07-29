package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dailyquestionapp "mathstudy/backend/internal/application/dailyquestion"
	exerciseapp "mathstudy/backend/internal/application/exercise"
	wechatreminder "mathstudy/backend/internal/application/wechatreminder"
	"mathstudy/backend/internal/platform/metautil"
	"mathstudy/backend/internal/platform/questiondedupe"
)

// DailyQuestionRepository persists daily assignments, selection policy, and reminders.
type DailyQuestionRepository struct {
	Repository
	wechatReminders WechatReminderEnqueuer
}

// NewDailyQuestionRepository creates a PostgreSQL-backed daily-question repository.
func NewDailyQuestionRepository(db Querier, reminders ...WechatReminderEnqueuer) (DailyQuestionRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return DailyQuestionRepository{}, err
	}
	var reminderEnqueuer WechatReminderEnqueuer
	if len(reminders) > 0 {
		reminderEnqueuer = reminders[0]
	}
	return DailyQuestionRepository{Repository: base, wechatReminders: reminderEnqueuer}, nil
}

// GetAssignment returns one student-day assignment with its frozen question snapshot.
func (r DailyQuestionRepository) GetAssignment(ctx context.Context, studentID string, date time.Time) (dailyquestionapp.Assignment, bool, error) {
	row := r.DB().QueryRow(ctx, assignmentSelectSQL+`
		WHERE a.student_id = $1 AND a.assignment_date = $2::date`, studentID, date)
	return scanOptionalDailyAssignment(row)
}

// ListAssignments returns persisted assignments in reverse calendar order.
func (r DailyQuestionRepository) ListAssignments(ctx context.Context, studentID string, start, end time.Time) ([]dailyquestionapp.Assignment, error) {
	rows, err := r.DB().Query(ctx, assignmentSelectSQL+`
		WHERE a.student_id = $1
		  AND a.assignment_date >= $2::date
		  AND a.assignment_date < $3::date
		ORDER BY a.assignment_date DESC`, studentID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dailyquestionapp.Assignment, 0)
	for rows.Next() {
		item, err := scanDailyAssignment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListStreakDates returns only on-time completed assignment dates.
func (r DailyQuestionRepository) ListStreakDates(ctx context.Context, studentID string, start, end time.Time) ([]time.Time, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT assignment_date
		FROM public.daily_question_assignments
		WHERE student_id = $1
		  AND assignment_date >= $2::date
		  AND assignment_date <= $3::date
		  AND status = 'completed'
		  AND counts_toward_streak = true
		ORDER BY assignment_date DESC`, studentID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	dates := make([]time.Time, 0)
	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

// ListActiveStudentIDs returns one stable keyset page of enabled student accounts.
func (r DailyQuestionRepository) ListActiveStudentIDs(ctx context.Context, afterID string, limit int) ([]string, error) {
	if limit < 1 {
		return nil, errors.New("active student page size must be positive")
	}
	rows, err := r.DB().Query(ctx, `
		SELECT account.id
		FROM public.users account
		WHERE account.id > $1
		  AND account.role = 'STUDENT'::public.userrole
		  AND account.status = 'ACTIVE'::public.userstatus
		  AND account.is_active = true
		ORDER BY account.id
		LIMIT $2`, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	studentIDs := make([]string, 0, limit)
	for rows.Next() {
		var studentID string
		if err := rows.Scan(&studentID); err != nil {
			return nil, err
		}
		studentIDs = append(studentIDs, studentID)
	}
	return studentIDs, rows.Err()
}

// GetStudentContext returns a current enrollment and the strategy locked for the requested day.
func (r DailyQuestionRepository) GetStudentContext(ctx context.Context, studentID string, date time.Time) (dailyquestionapp.StudentContext, error) {
	var classID pgtype.Text
	var teacherID pgtype.Text
	var strategy string
	var difficulty float64
	err := r.DB().QueryRow(ctx, `
		SELECT ce.class_id,
		       c.teacher_id,
		       coalesce(
		           (
		               SELECT CASE
		                          WHEN assignment.selection_reason = 'teacher_uniform' THEN 'uniform'
		                          ELSE 'personalized'
		                      END
		               FROM public.daily_question_assignments assignment
		               WHERE assignment.class_id = ce.class_id
		                 AND assignment.assignment_date = $2::date
		               ORDER BY assignment.assigned_at, assignment.id
		               LIMIT 1
		           ),
		           settings.strategy,
		           'personalized'
		       ),
		       coalesce(profile.preferred_difficulty, 0.5)::double precision
		FROM (SELECT $1::varchar AS student_id) seed
		LEFT JOIN public.class_enrollments ce ON ce.student_id = seed.student_id
		LEFT JOIN public.classes c ON c.id = ce.class_id
		LEFT JOIN public.daily_question_class_settings settings ON settings.class_id = c.id
		LEFT JOIN public.student_profiles profile ON profile.student_id = seed.student_id`, studentID, date).Scan(
		&classID, &teacherID, &strategy, &difficulty,
	)
	if err != nil {
		return dailyquestionapp.StudentContext{}, err
	}
	result := dailyquestionapp.StudentContext{ClassStrategy: strategy, PreferredDifficulty: difficulty}
	if classID.Valid {
		result.ClassID = classID.String
	}
	if teacherID.Valid {
		result.TeacherID = teacherID.String
	}
	return result, nil
}

// SelectTargetConcept applies mistake, learning-goal, weakest-state, then default priority.
func (r DailyQuestionRepository) SelectTargetConcept(ctx context.Context, studentID string) (dailyquestionapp.TargetSelection, bool, error) {
	queries := []struct {
		reason string
		sql    string
	}{
		{
			reason: dailyquestionapp.ReasonMistakeReview,
			sql: `
				SELECT concept.value
				FROM public.content_attempts attempt
				JOIN public.contents content ON content.id = attempt.content_id
				CROSS JOIN LATERAL json_array_elements_text(content.concept_ids) concept(value)
				JOIN public.knowledge_nodes node ON node.id = concept.value
				WHERE attempt.student_id = $1
				  AND attempt.submitted_at IS NOT NULL
				  AND attempt.is_correct = false
				  AND NOT EXISTS (
				      SELECT 1
				      FROM public.content_attempts corrected
				      WHERE corrected.student_id = attempt.student_id
				        AND corrected.content_id = attempt.content_id
				        AND corrected.is_correct = true
				        AND corrected.submitted_at > attempt.submitted_at
				  )
				ORDER BY attempt.submitted_at DESC, attempt.id DESC, concept.value
				LIMIT 1`,
		},
		{
			reason: dailyquestionapp.ReasonLearningGoal,
			sql: `
				SELECT goal.target_node_id
				FROM public.student_learning_goals goal
				JOIN public.knowledge_nodes node ON node.id = goal.target_node_id
				WHERE goal.student_id = $1`,
		},
		{
			reason: dailyquestionapp.ReasonWeakest,
			sql: `
				SELECT state.concept_id
				FROM public.student_concept_dkt_states state
				JOIN public.knowledge_nodes node ON node.id = state.concept_id
				WHERE state.student_id = $1
				ORDER BY state.mastery_prob ASC, state.confidence DESC,
				         state.last_attempt_at DESC NULLS LAST, state.concept_id
				LIMIT 1`,
		},
		{
			reason: dailyquestionapp.ReasonDefault,
			sql: `
				SELECT node.id
				FROM public.knowledge_nodes node
				WHERE $1::varchar IS NOT NULL
				ORDER BY node.difficulty ASC, node.id
				LIMIT 1`,
		},
	}
	for _, candidate := range queries {
		var conceptID string
		err := r.DB().QueryRow(ctx, candidate.sql, studentID).Scan(&conceptID)
		if err == nil {
			return dailyquestionapp.TargetSelection{ConceptID: conceptID, Reason: candidate.reason}, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return dailyquestionapp.TargetSelection{}, false, err
		}
	}
	return dailyquestionapp.TargetSelection{}, false, nil
}

// ListHistoricalQuestionBodies returns the frozen stems assigned before the requested day.
func (r DailyQuestionRepository) ListHistoricalQuestionBodies(ctx context.Context, studentID string, beforeDate time.Time) ([]string, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT coalesce(assignment.question_body, content.body)
		FROM public.daily_question_assignments assignment
		LEFT JOIN public.contents content
		  ON content.id = assignment.content_id
		 AND content.type = 'PROBLEM'::public.contenttype
		WHERE assignment.student_id = $1
		  AND assignment.assignment_date < $2::date
		  AND BTRIM(coalesce(assignment.question_body, content.body, '')) <> ''
		ORDER BY assignment.assignment_date DESC, assignment.id DESC`, studentID, beforeDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bodies := make([]string, 0)
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		bodies = append(bodies, body)
	}
	return bodies, rows.Err()
}

// ListRecentAttemptedContentIDs returns the student's latest submitted attempts across all exercise flows.
func (r DailyQuestionRepository) ListRecentAttemptedContentIDs(ctx context.Context, studentID string, limit int) ([]string, error) {
	if limit < 1 {
		return []string{}, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT attempt.content_id
		FROM public.content_attempts attempt
		WHERE attempt.student_id = $1
		  AND attempt.submitted_at IS NOT NULL
		ORDER BY attempt.submitted_at DESC, attempt.id DESC
		LIMIT $2`, studentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contentIDs := make([]string, 0, limit)
	for rows.Next() {
		var contentID string
		if err := rows.Scan(&contentID); err != nil {
			return nil, err
		}
		contentIDs = append(contentIDs, contentID)
	}
	return contentIDs, rows.Err()
}

// FindTeacherContent chooses an unused daily-candidate or published-bank question.
func (r DailyQuestionRepository) FindTeacherContent(
	ctx context.Context,
	teacherID string,
	targetConceptID string,
	date time.Time,
	dailyCandidateOnly bool,
	studentID string,
	historicalBodies []string,
	excludedContentIDs []string,
) (dailyquestionapp.ContentChoice, bool, error) {
	query := `
		SELECT content.id, content.concept_ids, content.body
		FROM public.contents content
		WHERE content.owner_teacher_id = $1
		  AND content.type = 'PROBLEM'::public.contenttype
		  AND content.status = 'PUBLISHED'::public.contentstatus
		  AND content.deleted_at IS NULL
		  AND $3::date IS NOT NULL
		  AND (coalesce(cardinality($5::varchar[]), 0) = 0 OR NOT (content.id = ANY($5::varchar[])))
		  AND ($4 = '' OR EXISTS (
		      SELECT 1 FROM json_array_elements_text(content.concept_ids) concept(value)
		      WHERE concept.value = $4
		  ))
		  AND NOT EXISTS (
		      SELECT 1
		      FROM public.daily_question_assignments previous
		      LEFT JOIN public.contents previous_content ON previous_content.id = previous.content_id
		      WHERE previous.student_id = $2
		        AND (
			previous.content_id = content.id
			OR lower(regexp_replace(coalesce(previous.question_body, previous_content.body, ''), '[[:space:]]+', '', 'g')) =
			   lower(regexp_replace(content.body, '[[:space:]]+', '', 'g'))
		        )
		  )`
	if dailyCandidateOnly {
		query = `
			SELECT content.id, content.concept_ids, content.body
			FROM public.daily_question_candidates candidate
			JOIN public.contents content ON content.id = candidate.content_id
			WHERE candidate.teacher_id = $1
			  AND candidate.is_active = true
			  AND (candidate.valid_from IS NULL OR candidate.valid_from <= $3::date)
			  AND (candidate.valid_until IS NULL OR candidate.valid_until >= $3::date)
			  AND content.owner_teacher_id = $1
			  AND content.type = 'PROBLEM'::public.contenttype
			  AND content.status = 'PUBLISHED'::public.contentstatus
			  AND content.deleted_at IS NULL
			  AND (coalesce(cardinality($5::varchar[]), 0) = 0 OR NOT (content.id = ANY($5::varchar[])))
			  AND ($4 = '' OR EXISTS (
			      SELECT 1 FROM json_array_elements_text(content.concept_ids) concept(value)
			      WHERE concept.value = $4
			  ))
			  AND NOT EXISTS (
			      SELECT 1
			      FROM public.daily_question_assignments previous
			      LEFT JOIN public.contents previous_content ON previous_content.id = previous.content_id
			      WHERE previous.student_id = $2
			        AND (
			previous.content_id = content.id
			OR lower(regexp_replace(coalesce(previous.question_body, previous_content.body, ''), '[[:space:]]+', '', 'g')) =
			   lower(regexp_replace(content.body, '[[:space:]]+', '', 'g'))
			        )
			  )
			ORDER BY
			  CASE WHEN $4 = '' OR EXISTS (
			      SELECT 1 FROM json_array_elements_text(content.concept_ids) concept(value)
			      WHERE concept.value = $4
			  ) THEN 0 ELSE 1 END,
			  candidate.priority DESC,
			  content.difficulty ASC,
			  content.id`
	} else {
		query += `
			ORDER BY
			  CASE WHEN $4 = '' OR EXISTS (
			      SELECT 1 FROM json_array_elements_text(content.concept_ids) concept(value)
			      WHERE concept.value = $4
			  ) THEN 0 ELSE 1 END,
			  content.difficulty ASC,
			  content.id`
	}
	rows, err := r.DB().Query(ctx, query, teacherID, studentID, date, targetConceptID, excludedContentIDs)
	if err != nil {
		return dailyquestionapp.ContentChoice{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var contentID string
		var conceptIDsRaw []byte
		var body string
		if err := rows.Scan(&contentID, &conceptIDsRaw, &body); err != nil {
			return dailyquestionapp.ContentChoice{}, false, err
		}
		if questiondedupe.IsDuplicate(body, historicalBodies) {
			continue
		}
		conceptIDs, err := decodeStringSlice(conceptIDsRaw)
		if err != nil {
			return dailyquestionapp.ContentChoice{}, false, fmt.Errorf("decode daily content concept ids: %w", err)
		}
		selectedConcept := targetConceptID
		if !containsString(conceptIDs, selectedConcept) {
			selectedConcept = ""
			if len(conceptIDs) > 0 {
				selectedConcept = conceptIDs[0]
			}
		}
		return dailyquestionapp.ContentChoice{ContentID: contentID, TargetConceptID: selectedConcept}, true, nil
	}
	if err := rows.Err(); err != nil {
		return dailyquestionapp.ContentChoice{}, false, err
	}
	return dailyquestionapp.ContentChoice{}, false, nil
}

// FindTeacherRepeatFallback chooses the least recently assigned matching teacher question.
func (r DailyQuestionRepository) FindTeacherRepeatFallback(
	ctx context.Context,
	teacherID string,
	targetConceptID string,
	date time.Time,
	dailyCandidateOnly bool,
	studentID string,
	excludedContentIDs []string,
) (dailyquestionapp.ContentChoice, bool, error) {
	query := `
		WITH historical AS (
			SELECT assignment.content_id, max(assignment.assignment_date) AS last_assigned_date
			FROM public.daily_question_assignments assignment
			WHERE assignment.student_id = $2
			  AND assignment.content_id IS NOT NULL
			  AND assignment.assignment_date < $3::date
			GROUP BY assignment.content_id
		)
		SELECT content.id, content.concept_ids
		FROM historical
		JOIN public.contents content ON content.id = historical.content_id
		WHERE content.owner_teacher_id = $1
		  AND content.type = 'PROBLEM'::public.contenttype
		  AND content.status = 'PUBLISHED'::public.contentstatus
		  AND content.deleted_at IS NULL
		  AND $3::date IS NOT NULL
		  AND ($4 = '' OR EXISTS (
		      SELECT 1 FROM json_array_elements_text(content.concept_ids) concept(value)
		      WHERE concept.value = $4
		  ))
		ORDER BY
		  CASE WHEN coalesce(cardinality($5::varchar[]), 0) > 0 AND content.id = ANY($5::varchar[]) THEN 1 ELSE 0 END,
		  historical.last_assigned_date ASC, content.difficulty ASC, content.id
		LIMIT 1`
	if dailyCandidateOnly {
		query = `
			WITH historical AS (
				SELECT assignment.content_id, max(assignment.assignment_date) AS last_assigned_date
				FROM public.daily_question_assignments assignment
				WHERE assignment.student_id = $2
				  AND assignment.content_id IS NOT NULL
				  AND assignment.assignment_date < $3::date
				GROUP BY assignment.content_id
			)
			SELECT content.id, content.concept_ids
			FROM historical
			JOIN public.daily_question_candidates candidate ON candidate.content_id = historical.content_id
			JOIN public.contents content ON content.id = historical.content_id
			WHERE candidate.teacher_id = $1
			  AND candidate.is_active = true
			  AND (candidate.valid_from IS NULL OR candidate.valid_from <= $3::date)
			  AND (candidate.valid_until IS NULL OR candidate.valid_until >= $3::date)
			  AND content.owner_teacher_id = $1
			  AND content.type = 'PROBLEM'::public.contenttype
			  AND content.status = 'PUBLISHED'::public.contentstatus
			  AND content.deleted_at IS NULL
			  AND ($4 = '' OR EXISTS (
		      SELECT 1 FROM json_array_elements_text(content.concept_ids) concept(value)
		      WHERE concept.value = $4
		  ))
			ORDER BY
			  CASE WHEN coalesce(cardinality($5::varchar[]), 0) > 0 AND content.id = ANY($5::varchar[]) THEN 1 ELSE 0 END,
			  historical.last_assigned_date ASC, candidate.priority DESC, content.difficulty ASC, content.id
			LIMIT 1`
	}
	var contentID string
	var conceptIDsRaw []byte
	err := r.DB().QueryRow(ctx, query, teacherID, studentID, date, targetConceptID, excludedContentIDs).Scan(&contentID, &conceptIDsRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return dailyquestionapp.ContentChoice{}, false, nil
	}
	if err != nil {
		return dailyquestionapp.ContentChoice{}, false, err
	}
	conceptIDs, err := decodeStringSlice(conceptIDsRaw)
	if err != nil {
		return dailyquestionapp.ContentChoice{}, false, fmt.Errorf("decode daily repeat fallback concept ids: %w", err)
	}
	selectedConcept := targetConceptID
	if !containsString(conceptIDs, selectedConcept) {
		selectedConcept = ""
		if len(conceptIDs) > 0 {
			selectedConcept = conceptIDs[0]
		}
	}
	return dailyquestionapp.ContentChoice{ContentID: contentID, TargetConceptID: selectedConcept}, true, nil
}

// GetClassSelection returns a valid persisted uniform class-day question.
func (r DailyQuestionRepository) GetClassSelection(ctx context.Context, classID string, date time.Time) (dailyquestionapp.ClassSelection, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT selection.class_id,
		       selection.assignment_date,
		       selection.content_id,
		       selection.target_concept_id,
		       selection.source,
		       selection.selection_reason,
		       content.body
		FROM public.daily_question_class_selections selection
		JOIN public.contents content ON content.id = selection.content_id
		WHERE selection.class_id = $1
		  AND selection.assignment_date = $2::date
		  AND selection.source = 'teacher_bank'
		  AND selection.selection_reason = 'teacher_uniform'
		  AND content.type = 'PROBLEM'::public.contenttype
		  AND content.status = 'PUBLISHED'::public.contentstatus
		  AND content.deleted_at IS NULL`, classID, date)
	return scanOptionalClassSelection(row)
}

// GetClassUniformSchedule returns the explicit teacher schedule from start onward.
func (r DailyQuestionRepository) GetClassUniformSchedule(
	ctx context.Context,
	teacherID string,
	classID string,
	start time.Time,
	limit int,
) (dailyquestionapp.ClassUniformSchedule, bool, error) {
	var ownedClassID string
	if err := r.DB().QueryRow(ctx, `
		SELECT id
		FROM public.classes
		WHERE id = $1 AND teacher_id = $2`, classID, teacherID).Scan(&ownedClassID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dailyquestionapp.ClassUniformSchedule{}, false, nil
		}
		return dailyquestionapp.ClassUniformSchedule{}, false, err
	}
	if limit < 1 || limit > dailyquestionapp.MaxUniformScheduleItems {
		limit = dailyquestionapp.MaxUniformScheduleItems
	}

	rows, err := r.DB().Query(ctx, `
		SELECT selection.assignment_date,
		       selection.content_id,
		       selection.target_concept_id,
		       content.title,
		       content.body,
		       content.difficulty::double precision,
		       EXISTS (
		           SELECT 1
		           FROM public.daily_question_assignments assignment
		           WHERE assignment.class_id = selection.class_id
		             AND assignment.assignment_date = selection.assignment_date
		             AND (assignment.status <> 'unavailable' OR assignment.content_id IS NOT NULL)
		       ) AS locked
		FROM public.daily_question_class_selections selection
		JOIN public.contents content ON content.id = selection.content_id
		WHERE selection.class_id = $1
		  AND selection.assignment_date >= $2::date
		  AND selection.source = 'teacher_bank'
		  AND selection.selection_reason = 'teacher_uniform'
		ORDER BY selection.assignment_date ASC
		LIMIT $3`, classID, start, limit)
	if err != nil {
		return dailyquestionapp.ClassUniformSchedule{}, false, err
	}
	defer rows.Close()

	schedule := dailyquestionapp.ClassUniformSchedule{
		ClassID: classID,
		Items:   make([]dailyquestionapp.ClassUniformScheduleItem, 0),
	}
	for rows.Next() {
		var item dailyquestionapp.ClassUniformScheduleItem
		var assignmentDate time.Time
		var targetConceptID pgtype.Text
		if err := rows.Scan(
			&assignmentDate,
			&item.ContentID,
			&targetConceptID,
			&item.Title,
			&item.Body,
			&item.Difficulty,
			&item.Locked,
		); err != nil {
			return dailyquestionapp.ClassUniformSchedule{}, false, err
		}
		item.AssignmentDate = assignmentDate.Format("2006-01-02")
		item.TargetConceptID = textPointer(targetConceptID)
		schedule.Items = append(schedule.Items, item)
	}
	if err := rows.Err(); err != nil {
		return dailyquestionapp.ClassUniformSchedule{}, false, err
	}
	return schedule, true, nil
}

type uniformScheduleContent struct {
	targetConceptID *string
	status          string
	body            string
}

type uniformScheduleSelection struct {
	contentID *string
}

type classQuestionHistory struct {
	contentIDs map[string]struct{}
	bodies     []string
}

// ReplaceClassUniformSchedule validates, publishes, and reorders a class plan atomically.
func (r DailyQuestionRepository) ReplaceClassUniformSchedule(
	ctx context.Context,
	input dailyquestionapp.UniformScheduleInput,
) (dailyquestionapp.ClassUniformSchedule, bool, error) {
	var schedule dailyquestionapp.ClassUniformSchedule
	found := false
	err := withRepositoryTx(ctx, "replace daily question uniform schedule", r.Repository, func(base Repository) DailyQuestionRepository {
		return DailyQuestionRepository{Repository: base}
	}, func(tx DailyQuestionRepository) error {
		var classID string
		if err := tx.DB().QueryRow(ctx, `
			SELECT id
			FROM public.classes
			WHERE id = $1 AND teacher_id = $2
			FOR UPDATE`, input.ClassID, input.TeacherID).Scan(&classID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		found = true

		contents, err := tx.loadUniformScheduleContents(ctx, input.TeacherID, input.ContentIDs)
		if err != nil {
			return err
		}
		if len(contents) != len(input.ContentIDs) {
			return dailyquestionapp.ErrInvalidContent
		}
		historical, err := tx.listClassHistoricalQuestionBodies(ctx, input.ClassID, input.StartDate)
		if err != nil {
			return err
		}
		if hasDuplicateUniformQuestionBodies(input.ContentIDs, contents, historical) {
			return dailyquestionapp.ErrDuplicateQuestion
		}

		existing, err := tx.lockUniformScheduleSelections(ctx, input.ClassID, input.StartDate)
		if err != nil {
			return err
		}
		lockedDates, err := tx.listLockedUniformScheduleDates(ctx, input.ClassID, input.StartDate)
		if err != nil {
			return err
		}

		desired := make(map[string]string, len(input.ContentIDs))
		for index, contentID := range input.ContentIDs {
			date := input.StartDate.AddDate(0, 0, index).Format("2006-01-02")
			desired[date] = contentID
		}
		for date := range lockedDates {
			selection, selected := existing[date]
			desiredContentID, scheduled := desired[date]
			if !selected || selection.contentID == nil || !scheduled || *selection.contentID != desiredContentID {
				return dailyquestionapp.ErrSelectionLocked
			}
		}

		draftIDs := make([]string, 0)
		for _, contentID := range input.ContentIDs {
			if contents[contentID].status == "DRAFT" {
				draftIDs = append(draftIDs, contentID)
			}
		}
		if len(draftIDs) > 0 {
			questionRepo := QuestionRepository{Repository: tx.Repository}
			published, err := questionRepo.BatchPublish(ctx, input.TeacherID, draftIDs, input.Now)
			if err != nil {
				return err
			}
			if published != len(draftIDs) {
				return dailyquestionapp.ErrInvalidContent
			}
		}

		if _, err := tx.DB().Exec(ctx, `
			DELETE FROM public.daily_question_class_selections selection
			WHERE selection.class_id = $1
			  AND selection.assignment_date >= $2::date
				  AND NOT EXISTS (
				      SELECT 1
				      FROM public.daily_question_assignments assignment
				      WHERE assignment.class_id = selection.class_id
				        AND assignment.assignment_date = selection.assignment_date
				        AND (assignment.status <> 'unavailable' OR assignment.content_id IS NOT NULL)
				  )`, input.ClassID, input.StartDate); err != nil {
			return err
		}

		for index, contentID := range input.ContentIDs {
			date := input.StartDate.AddDate(0, 0, index)
			content := contents[contentID]
			if _, err := tx.DB().Exec(ctx, `
				INSERT INTO public.daily_question_class_selections (
					class_id, assignment_date, content_id, target_concept_id,
					source, selection_reason, created_at, updated_at
				)
				VALUES ($1, $2::date, $3, $4, 'teacher_bank', 'teacher_uniform', $5, $5)
				ON CONFLICT (class_id, assignment_date) DO UPDATE SET
					content_id = EXCLUDED.content_id,
					target_concept_id = EXCLUDED.target_concept_id,
					source = EXCLUDED.source,
					selection_reason = EXCLUDED.selection_reason,
					updated_at = EXCLUDED.updated_at`,
				input.ClassID,
				date,
				contentID,
				content.targetConceptID,
				input.Now,
			); err != nil {
				return err
			}
		}

		var desiredStrategy string
		if err := tx.DB().QueryRow(ctx, `
			SELECT coalesce(settings.strategy, 'personalized')
			FROM public.classes class
			LEFT JOIN public.daily_question_class_settings settings ON settings.class_id = class.id
			WHERE class.id = $1`, input.ClassID).Scan(&desiredStrategy); err != nil {
			return err
		}
		if desiredStrategy == dailyquestionapp.StrategyUniform {
			if _, assigned, err := tx.GetClassSelection(ctx, input.ClassID, input.StartDate); err != nil {
				return err
			} else if !assigned {
				return dailyquestionapp.ErrUniformQuestionNotAssigned
			}
		}

		stored, ok, err := tx.GetClassUniformSchedule(
			ctx,
			input.TeacherID,
			input.ClassID,
			input.StartDate,
			dailyquestionapp.MaxUniformScheduleItems,
		)
		if err != nil {
			return err
		}
		if !ok {
			return pgx.ErrNoRows
		}
		schedule = stored
		return nil
	})
	if err != nil {
		return dailyquestionapp.ClassUniformSchedule{}, false, err
	}
	return schedule, found, nil
}

func (r DailyQuestionRepository) loadUniformScheduleContents(
	ctx context.Context,
	teacherID string,
	contentIDs []string,
) (map[string]uniformScheduleContent, error) {
	result := make(map[string]uniformScheduleContent, len(contentIDs))
	if len(contentIDs) == 0 {
		return result, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT content.id,
		       content.status::text,
		       content.body,
		       (
		           SELECT node.id
		           FROM json_array_elements_text(content.concept_ids) WITH ORDINALITY concept(value, position)
		           JOIN public.knowledge_nodes node ON node.id = concept.value
		           ORDER BY concept.position
		           LIMIT 1
		       ) AS target_concept_id
		FROM public.contents content
		WHERE content.id = ANY($1::varchar[])
		  AND content.owner_teacher_id = $2
		  AND content.type = 'PROBLEM'::public.contenttype
		  AND content.status IN ('DRAFT'::public.contentstatus, 'PUBLISHED'::public.contentstatus)
		  AND content.deleted_at IS NULL
		ORDER BY content.id
		FOR UPDATE`, contentIDs, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var contentID string
		var status string
		var targetConceptID pgtype.Text
		var body string
		if err := rows.Scan(&contentID, &status, &body, &targetConceptID); err != nil {
			return nil, err
		}
		result[contentID] = uniformScheduleContent{
			targetConceptID: textPointer(targetConceptID),
			status:          status,
			body:            body,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func hasDuplicateUniformQuestionBodies(contentIDs []string, contents map[string]uniformScheduleContent, history classQuestionHistory) bool {
	bodies := append([]string(nil), history.bodies...)
	for _, contentID := range contentIDs {
		content, ok := contents[contentID]
		if !ok {
			return false
		}
		if _, exists := history.contentIDs[contentID]; exists {
			return true
		}
		if questiondedupe.IsDuplicate(content.body, bodies) {
			return true
		}
		bodies = append(bodies, content.body)
	}
	return false
}

// listClassHistoricalQuestionBodies returns daily questions previously assigned to current class members,
// including work they received before entering this class.
func (r DailyQuestionRepository) listClassHistoricalQuestionBodies(ctx context.Context, classID string, beforeDate time.Time) (classQuestionHistory, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT DISTINCT assignment.content_id,
		       coalesce(assignment.question_body, content.body)
		FROM public.class_enrollments enrollment
		JOIN public.daily_question_assignments assignment ON assignment.student_id = enrollment.student_id
		LEFT JOIN public.contents content
		  ON content.id = assignment.content_id
		 AND content.type = 'PROBLEM'::public.contenttype
		WHERE enrollment.class_id = $1
		  AND assignment.assignment_date < $2::date
		  AND BTRIM(coalesce(assignment.question_body, content.body, '')) <> ''
		ORDER BY coalesce(assignment.question_body, content.body)`, classID, beforeDate)
	if err != nil {
		return classQuestionHistory{}, err
	}
	defer rows.Close()

	history := classQuestionHistory{
		contentIDs: make(map[string]struct{}),
		bodies:     make([]string, 0),
	}
	for rows.Next() {
		var contentID pgtype.Text
		var body string
		if err := rows.Scan(&contentID, &body); err != nil {
			return classQuestionHistory{}, err
		}
		if contentID.Valid {
			history.contentIDs[contentID.String] = struct{}{}
		}
		history.bodies = append(history.bodies, body)
	}
	if err := rows.Err(); err != nil {
		return classQuestionHistory{}, err
	}
	return history, nil
}

func (r DailyQuestionRepository) lockUniformScheduleSelections(
	ctx context.Context,
	classID string,
	start time.Time,
) (map[string]uniformScheduleSelection, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT selection.assignment_date,
		       selection.content_id
		FROM public.daily_question_class_selections selection
		WHERE selection.class_id = $1
		  AND selection.assignment_date >= $2::date
		ORDER BY selection.assignment_date
		FOR UPDATE`, classID, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]uniformScheduleSelection)
	for rows.Next() {
		var date time.Time
		var contentID pgtype.Text
		if err := rows.Scan(&date, &contentID); err != nil {
			return nil, err
		}
		result[date.Format("2006-01-02")] = uniformScheduleSelection{contentID: textPointer(contentID)}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r DailyQuestionRepository) listLockedUniformScheduleDates(
	ctx context.Context,
	classID string,
	start time.Time,
) (map[string]struct{}, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT DISTINCT assignment.assignment_date
		FROM public.daily_question_assignments assignment
		WHERE assignment.class_id = $1
		  AND assignment.assignment_date >= $2::date
		  AND (assignment.status <> 'unavailable' OR assignment.content_id IS NOT NULL)
		ORDER BY assignment.assignment_date`, classID, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]struct{})
	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		result[date.Format("2006-01-02")] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ReservePreparation creates the unique day row or takes over a failed/stale row.
func (r DailyQuestionRepository) ReservePreparation(ctx context.Context, input dailyquestionapp.PreparationReservation) (dailyquestionapp.Assignment, bool, error) {
	var assignment dailyquestionapp.Assignment
	claimed := false
	err := withRepositoryTx(ctx, "reserve daily question preparation", r.Repository, func(base Repository) DailyQuestionRepository {
		return DailyQuestionRepository{Repository: base}
	}, func(tx DailyQuestionRepository) error {
		if input.ClassID != "" {
			var classID string
			if err := tx.DB().QueryRow(ctx, `
				SELECT id
				FROM public.classes
				WHERE id = $1
				FOR SHARE`, input.ClassID).Scan(&classID); err != nil {
				return err
			}

			var effectiveStrategy string
			if err := tx.DB().QueryRow(ctx, `
				SELECT coalesce(
				           (
				               SELECT CASE
				                          WHEN assignment.selection_reason = 'teacher_uniform' THEN 'uniform'
				                          ELSE 'personalized'
				                      END
				               FROM public.daily_question_assignments assignment
				               WHERE assignment.class_id = class.id
				                 AND assignment.assignment_date = $2::date
				               ORDER BY assignment.assigned_at, assignment.id
				               LIMIT 1
				           ),
				           settings.strategy,
				           'personalized'
				       )
				FROM public.classes class
				LEFT JOIN public.daily_question_class_settings settings ON settings.class_id = class.id
				WHERE class.id = $1`, input.ClassID, input.AssignmentDate).Scan(&effectiveStrategy); err != nil {
				return err
			}
			if effectiveStrategy != input.ClassStrategy {
				return dailyquestionapp.ErrStrategyChanged
			}
		}

		var returnedID string
		err := tx.DB().QueryRow(ctx, `
		INSERT INTO public.daily_question_assignments (
			id, student_id, class_id, assignment_date, content_id,
			target_concept_id, source, selection_reason, status,
			first_attempt_id, corrected_attempt_id, first_result,
			counts_toward_streak, generation_token, retry_count,
			failure_code, assigned_at, opened_at, completed_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4::date, NULL,
			$5, 'ai_generated', $6, 'preparing',
			NULL, NULL, NULL, false, $7, 0,
			NULL, $8, $9, NULL, $8
		)
		ON CONFLICT (student_id, assignment_date) DO UPDATE SET
			class_id = EXCLUDED.class_id,
			content_id = NULL,
			question_title = NULL,
			question_body = NULL,
			question_difficulty = NULL,
			question_concept_ids = NULL,
			question_meta = NULL,
			question_generated_by_student_id = NULL,
			target_concept_id = EXCLUDED.target_concept_id,
			source = 'ai_generated',
			selection_reason = EXCLUDED.selection_reason,
			status = 'preparing',
			generation_token = EXCLUDED.generation_token,
			retry_count = public.daily_question_assignments.retry_count + 1,
			failure_code = NULL,
			opened_at = coalesce(public.daily_question_assignments.opened_at, EXCLUDED.opened_at),
			updated_at = EXCLUDED.updated_at
		WHERE public.daily_question_assignments.status = 'unavailable'
		   OR (
		       public.daily_question_assignments.status = 'preparing'
		       AND public.daily_question_assignments.updated_at <= $10
		   )
		   OR (
		       public.daily_question_assignments.status = 'ready'
		       AND (
		           public.daily_question_assignments.content_id IS NULL
		           OR public.daily_question_assignments.question_body IS NULL
		       )
		   )
		RETURNING id`,
			input.AssignmentID,
			input.StudentID,
			nullableString(input.ClassID),
			input.AssignmentDate,
			nullableString(input.TargetConceptID),
			input.SelectionReason,
			input.GenerationToken,
			input.Now,
			input.OpenedAt,
			input.StaleBefore,
		).Scan(&returnedID)
		inserted := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		stored, found, err := tx.GetAssignment(ctx, input.StudentID, input.AssignmentDate)
		if err != nil {
			return err
		}
		if !found {
			return pgx.ErrNoRows
		}
		assignment = stored
		claimed = inserted && returnedID == stored.ID
		return nil
	})
	if err != nil {
		return dailyquestionapp.Assignment{}, false, err
	}
	return assignment, claimed, nil
}

// FinishPreparation publishes the selected content only for the active generation token.
func (r DailyQuestionRepository) FinishPreparation(
	ctx context.Context,
	assignmentID string,
	generationToken string,
	choice dailyquestionapp.ContentChoice,
	source string,
	reason string,
	now time.Time,
) (bool, error) {
	finished := false
	err := withRepositoryTx(ctx, "finish daily question preparation", r.Repository, func(base Repository) DailyQuestionRepository {
		return DailyQuestionRepository{Repository: base}
	}, func(tx DailyQuestionRepository) error {
		var studentID string
		var assignmentDate time.Time
		err := tx.DB().QueryRow(ctx, `
			SELECT student_id, assignment_date
			FROM public.daily_question_assignments
			WHERE id = $1
			  AND generation_token = $2
			  AND status = 'preparing'
			FOR UPDATE`, assignmentID, generationToken).Scan(&studentID, &assignmentDate)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		var snapshotTitle string
		var snapshotBody string
		var snapshotDifficulty float64
		var snapshotConceptIDs []byte
		var snapshotMeta []byte
		var snapshotGeneratedBy pgtype.Text
		err = tx.DB().QueryRow(ctx, `
			SELECT title,
			       body,
			       difficulty,
			       concept_ids,
			       meta,
			       generated_by_student_id
			FROM public.contents
			WHERE id = $1
			  AND type = 'PROBLEM'::public.contenttype
			  AND status = 'PUBLISHED'::public.contentstatus
			  AND deleted_at IS NULL
			FOR SHARE`, choice.ContentID).Scan(
			&snapshotTitle,
			&snapshotBody,
			&snapshotDifficulty,
			&snapshotConceptIDs,
			&snapshotMeta,
			&snapshotGeneratedBy,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		if reason != dailyquestionapp.ReasonRepeatFallback {
			var lockedStudentID string
			if err := tx.DB().QueryRow(ctx, `
				SELECT id
				FROM public.users
				WHERE id = $1
				FOR UPDATE`, studentID).Scan(&lockedStudentID); err != nil {
				return err
			}
			rows, err := tx.DB().Query(ctx, `
				SELECT previous.content_id,
				       coalesce(previous.question_body, content.body)
				FROM public.daily_question_assignments previous
				LEFT JOIN public.contents content
				  ON content.id = previous.content_id
				 AND content.type = 'PROBLEM'::public.contenttype
				WHERE previous.student_id = $1
				  AND previous.id <> $2
				  AND previous.assignment_date <> $3::date
				  AND previous.content_id IS NOT NULL
				  AND BTRIM(coalesce(previous.question_body, content.body, '')) <> ''`, studentID, assignmentID, assignmentDate)
			if err != nil {
				return err
			}
			historicalBodies := make([]string, 0)
			for rows.Next() {
				var historicalContentID string
				var historicalBody string
				if err := rows.Scan(&historicalContentID, &historicalBody); err != nil {
					rows.Close()
					return err
				}
				if historicalContentID == choice.ContentID {
					rows.Close()
					return dailyquestionapp.ErrDuplicateQuestion
				}
				historicalBodies = append(historicalBodies, historicalBody)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return err
			}
			rows.Close()
			if questiondedupe.IsDuplicate(snapshotBody, historicalBodies) {
				return dailyquestionapp.ErrDuplicateQuestion
			}
		}

		tag, err := tx.DB().Exec(ctx, `
			UPDATE public.daily_question_assignments assignment
			SET content_id = $3,
			    target_concept_id = $4,
			    source = $5,
			    selection_reason = $6,
			    question_title = $7,
			    question_body = $8,
			    question_difficulty = $9,
			    question_concept_ids = $10::json,
			    question_meta = $11::json,
			    question_generated_by_student_id = $12,
			    status = 'ready',
			    generation_token = NULL,
			    failure_code = NULL,
			    updated_at = $13
			WHERE assignment.id = $1
			  AND assignment.generation_token = $2
			  AND assignment.status = 'preparing'`,
			assignmentID,
			generationToken,
			choice.ContentID,
			nullableString(choice.TargetConceptID),
			source,
			reason,
			snapshotTitle,
			snapshotBody,
			snapshotDifficulty,
			string(snapshotConceptIDs),
			string(snapshotMeta),
			nullableString(snapshotGeneratedBy.String),
			now,
		)
		if err != nil {
			return err
		}
		finished = tag.RowsAffected() > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	return finished, nil
}

// FailPreparation transitions only the active generation attempt to unavailable.
func (r DailyQuestionRepository) FailPreparation(ctx context.Context, assignmentID, generationToken, failureCode string, now time.Time) error {
	_, err := r.DB().Exec(ctx, `
		UPDATE public.daily_question_assignments
		SET status = 'unavailable',
		    generation_token = NULL,
		    failure_code = $3,
		    updated_at = $4
		WHERE id = $1
		  AND generation_token = $2
		  AND status = 'preparing'`, assignmentID, generationToken, failureCode, now)
	return err
}

// MarkOpened records first entry without extending a preparing row's recovery window.
func (r DailyQuestionRepository) MarkOpened(ctx context.Context, studentID string, date, now time.Time) (dailyquestionapp.Assignment, bool, error) {
	_, err := r.DB().Exec(ctx, `
		UPDATE public.daily_question_assignments
		SET opened_at = coalesce(opened_at, $3),
		    updated_at = CASE WHEN status = 'preparing' THEN updated_at ELSE $3 END
		WHERE student_id = $1
		  AND assignment_date = $2::date
		  AND status IN ('preparing', 'ready', 'completed')`, studentID, date, now)
	if err != nil {
		return dailyquestionapp.Assignment{}, false, err
	}
	return r.GetAssignment(ctx, studentID, date)
}

// GetClassSettings returns both the teacher's selected strategy and today's locked strategy.
func (r DailyQuestionRepository) GetClassSettings(ctx context.Context, teacherID, classID string, date time.Time) (dailyquestionapp.ClassSettings, bool, error) {
	var settings dailyquestionapp.ClassSettings
	var effectiveDate time.Time
	err := r.DB().QueryRow(ctx, `
		WITH state AS (
			SELECT class.id,
			       class.teacher_id,
			       coalesce(settings.strategy, 'personalized') AS desired_strategy,
			       coalesce(settings.auto_reminder_enabled, false) AS auto_reminder_enabled,
			       (
			           SELECT CASE
			                      WHEN assignment.selection_reason = 'teacher_uniform' THEN 'uniform'
			                      ELSE 'personalized'
			                  END
			           FROM public.daily_question_assignments assignment
			           WHERE assignment.class_id = class.id
			             AND assignment.assignment_date = $3::date
			           ORDER BY assignment.assigned_at, assignment.id
			           LIMIT 1
			       ) AS locked_strategy,
			       (
			           SELECT count(*)::int
			           FROM public.daily_question_assignments assignment
			           WHERE assignment.class_id = class.id
			             AND assignment.assignment_date = $3::date
			       ) AS assignment_count
			FROM public.classes class
			LEFT JOIN public.daily_question_class_settings settings ON settings.class_id = class.id
			WHERE class.id = $1 AND class.teacher_id = $2
		)
		SELECT state.id,
		       state.teacher_id,
		       state.desired_strategy,
		       coalesce(state.locked_strategy, state.desired_strategy) AS effective_strategy,
		       CASE
		           WHEN state.locked_strategy IS NOT NULL
		            AND state.locked_strategy <> state.desired_strategy
		           THEN $3::date + 1
		           ELSE $3::date
		       END AS effective_date,
		       state.assignment_count,
		       EXISTS (
		           SELECT 1
		           FROM public.daily_question_class_selections selection
		           JOIN public.contents content ON content.id = selection.content_id
		           WHERE selection.class_id = state.id
		             AND selection.assignment_date = CASE
		                 WHEN state.assignment_count > 0
		                  AND coalesce(state.locked_strategy, state.desired_strategy) <> 'uniform'
		                 THEN $3::date + 1
		                 ELSE $3::date
		             END
		             AND selection.source = 'teacher_bank'
		             AND selection.selection_reason = 'teacher_uniform'
		             AND content.type = 'PROBLEM'::public.contenttype
		             AND content.status = 'PUBLISHED'::public.contentstatus
		             AND content.deleted_at IS NULL
		       ) OR EXISTS (
		           SELECT 1
		           FROM public.daily_question_assignments assignment
		           WHERE assignment.class_id = state.id
		             AND assignment.assignment_date = CASE
		                 WHEN state.assignment_count > 0
		                  AND coalesce(state.locked_strategy, state.desired_strategy) <> 'uniform'
		                 THEN $3::date + 1
		                 ELSE $3::date
		             END
		             AND assignment.selection_reason = 'teacher_uniform'
		             AND assignment.status IN ('ready', 'completed')
		             AND assignment.question_body IS NOT NULL
		       ) AS uniform_ready,
		       state.auto_reminder_enabled,
		       EXISTS (
		           SELECT 1
		           FROM public.daily_question_wechat_events event
		           WHERE event.class_id = state.id
		             AND event.assignment_date = $3::date
		             AND event.kind IN ('manual_student_reminder', 'automatic_student_reminder')
		       ) AS today_reminder_sent,
		       coalesce((
		           SELECT event.recipient_count
		           FROM public.daily_question_wechat_events event
		           WHERE event.class_id = state.id
		             AND event.assignment_date = $3::date
		             AND event.kind IN ('manual_student_reminder', 'automatic_student_reminder')
		           ORDER BY event.created_at DESC, event.id DESC
		           LIMIT 1
		       ), 0)::int AS today_reminder_recipient_count
		FROM state`, classID, teacherID, date).Scan(
		&settings.ClassID,
		&settings.TeacherID,
		&settings.Strategy,
		&settings.EffectiveStrategy,
		&effectiveDate,
		&settings.TodayAssignmentCount,
		&settings.UniformReady,
		&settings.AutoReminderEnabled,
		&settings.TodayReminderSent,
		&settings.TodayReminderRecipientCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dailyquestionapp.ClassSettings{}, false, nil
	}
	settings.EffectiveDate = effectiveDate.Format("2006-01-02")
	return settings, err == nil, err
}

// UpsertClassSettings serializes teacher changes with student reservations.
func (r DailyQuestionRepository) UpsertClassSettings(
	ctx context.Context,
	settings dailyquestionapp.ClassSettings,
	date time.Time,
	now time.Time,
) (dailyquestionapp.ClassSettings, bool, error) {
	var result dailyquestionapp.ClassSettings
	found := false
	err := withRepositoryTx(ctx, "update daily question class strategy", r.Repository, func(base Repository) DailyQuestionRepository {
		return DailyQuestionRepository{Repository: base}
	}, func(tx DailyQuestionRepository) error {
		var classID string
		if err := tx.DB().QueryRow(ctx, `
			SELECT id
			FROM public.classes
			WHERE id = $1 AND teacher_id = $2
			FOR UPDATE`, settings.ClassID, settings.TeacherID).Scan(&classID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		found = true

		current, ok, err := tx.GetClassSettings(ctx, settings.TeacherID, settings.ClassID, date)
		if err != nil {
			return err
		}
		if !ok {
			return pgx.ErrNoRows
		}
		if current.Strategy == settings.Strategy && current.AutoReminderEnabled == settings.AutoReminderEnabled {
			result = current
			return nil
		}

		// A uniform schedule is required only when switching into uniform mode.
		// An exhausted schedule must not prevent the teacher from disabling an
		// unrelated automatic-reminder setting.
		if settings.Strategy == dailyquestionapp.StrategyUniform && current.Strategy != dailyquestionapp.StrategyUniform {
			activationDate := date
			if current.TodayAssignmentCount > 0 && current.EffectiveStrategy != dailyquestionapp.StrategyUniform {
				activationDate = activationDate.AddDate(0, 0, 1)
			}
			if _, assigned, err := tx.GetClassSelection(ctx, settings.ClassID, activationDate); err != nil {
				return err
			} else if !assigned {
				return dailyquestionapp.ErrUniformQuestionNotAssigned
			}
		}

		if _, err := tx.DB().Exec(ctx, `
			INSERT INTO public.daily_question_class_settings (
				class_id, teacher_id, strategy, auto_reminder_enabled, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (class_id) DO UPDATE SET
				teacher_id = EXCLUDED.teacher_id,
				strategy = EXCLUDED.strategy,
				auto_reminder_enabled = EXCLUDED.auto_reminder_enabled,
				updated_at = EXCLUDED.updated_at`,
			settings.ClassID,
			settings.TeacherID,
			settings.Strategy,
			settings.AutoReminderEnabled,
			now,
		); err != nil {
			return err
		}

		stored, ok, err := tx.GetClassSettings(ctx, settings.TeacherID, settings.ClassID, date)
		if err != nil {
			return err
		}
		if !ok {
			return pgx.ErrNoRows
		}
		result = stored
		return nil
	})
	if err != nil {
		return dailyquestionapp.ClassSettings{}, false, err
	}
	return result, found, nil
}

// GetClassStatistics calculates fair aggregate metrics and omits student ranking.
func (r DailyQuestionRepository) GetClassStatistics(ctx context.Context, teacherID, classID string, date time.Time) (dailyquestionapp.ClassStatistics, bool, error) {
	var result dailyquestionapp.ClassStatistics
	var firstIncorrectCount int
	err := r.DB().QueryRow(ctx, `
		WITH owned_class AS (
			SELECT id FROM public.classes WHERE id = $1 AND teacher_id = $2
		), current_enrolled AS (
			SELECT enrollment.student_id
			FROM public.class_enrollments enrollment
			JOIN owned_class ON owned_class.id = enrollment.class_id
		), assigned_students AS (
			SELECT assignment.student_id
			FROM public.daily_question_assignments assignment
			JOIN owned_class ON owned_class.id = assignment.class_id
			WHERE assignment.assignment_date = $3::date
		), roster AS (
			SELECT student_id FROM current_enrolled
			UNION
			SELECT student_id FROM assigned_students
		)
		SELECT owned_class.id,
		       count(roster.student_id)::int,
		       count(assignment.id)::int,
		       count(assignment.id) FILTER (WHERE assignment.status = 'completed')::int,
		       count(assignment.id) FILTER (WHERE assignment.first_result = 'correct')::int,
		       count(assignment.id) FILTER (WHERE assignment.first_result = 'incorrect')::int,
		       count(assignment.id) FILTER (WHERE assignment.corrected_attempt_id IS NOT NULL)::int
		FROM owned_class
		LEFT JOIN roster ON true
		LEFT JOIN public.daily_question_assignments assignment
		  ON assignment.student_id = roster.student_id
		 AND assignment.class_id = owned_class.id
		 AND assignment.assignment_date = $3::date
		GROUP BY owned_class.id`, classID, teacherID, date).Scan(
		&result.ClassID,
		&result.StudentCount,
		&result.AssignedCount,
		&result.CompletedCount,
		&result.FirstCorrectCount,
		&firstIncorrectCount,
		&result.CorrectedCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dailyquestionapp.ClassStatistics{}, false, nil
	}
	if err != nil {
		return dailyquestionapp.ClassStatistics{}, false, err
	}
	result.CompletionRate = percent(result.StudentCount, result.CompletedCount)
	result.FirstCorrectRate = percent(result.CompletedCount, result.FirstCorrectCount)
	result.CorrectionRate = percent(firstIncorrectCount, result.CorrectedCount)

	rows, err := r.DB().Query(ctx, `
		SELECT assignment.target_concept_id,
		       coalesce(node.name, assignment.target_concept_id),
		       count(*)::int
		FROM public.daily_question_assignments assignment
		JOIN public.classes class
		  ON class.id = assignment.class_id
		 AND class.teacher_id = $2
		LEFT JOIN public.knowledge_nodes node ON node.id = assignment.target_concept_id
		WHERE assignment.class_id = $1
		  AND assignment.assignment_date = $3::date
		  AND assignment.first_result = 'incorrect'
		  AND assignment.target_concept_id IS NOT NULL
		GROUP BY assignment.target_concept_id, node.name
		ORDER BY count(*) DESC, assignment.target_concept_id
		LIMIT 10`, classID, teacherID, date)
	if err != nil {
		return dailyquestionapp.ClassStatistics{}, false, err
	}
	defer rows.Close()
	result.WeakConcepts = make([]dailyquestionapp.WeakConcept, 0)
	for rows.Next() {
		var item dailyquestionapp.WeakConcept
		if err := rows.Scan(&item.ConceptID, &item.ConceptName, &item.WrongCount); err != nil {
			return dailyquestionapp.ClassStatistics{}, false, err
		}
		result.WeakConcepts = append(result.WeakConcepts, item)
	}
	if err := rows.Err(); err != nil {
		return dailyquestionapp.ClassStatistics{}, false, err
	}
	return result, true, nil
}

// CreateClassReminder creates a WeChat-only daily-question event for current
// students who still have a ready question. It never writes a public notice.
func (r DailyQuestionRepository) CreateClassReminder(ctx context.Context, input dailyquestionapp.ClassReminderInput) (dailyquestionapp.ReminderResult, bool, error) {
	if !r.wechatReminders.Enabled() {
		return dailyquestionapp.ReminderResult{}, false, dailyquestionapp.ErrReminderUnavailable
	}
	if input.Kind != dailyquestionapp.ReminderKindManualStudent && input.Kind != dailyquestionapp.ReminderKindAutomaticStudent {
		return dailyquestionapp.ReminderResult{}, false, errors.New("invalid daily question reminder kind")
	}
	var result dailyquestionapp.ReminderResult
	found := false
	err := withRepositoryTx(ctx, "daily question wechat reminder", r.Repository, func(base Repository) DailyQuestionRepository {
		return DailyQuestionRepository{Repository: base, wechatReminders: r.wechatReminders}
	}, func(tx DailyQuestionRepository) error {
		var classID string
		if err := tx.DB().QueryRow(ctx, `
			SELECT id
			FROM public.classes
			WHERE id = $1 AND teacher_id = $2
			FOR UPDATE`, input.ClassID, input.TeacherID).Scan(&classID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		found = true

		var existingID string
		var recipientCount int
		var hasActiveOrSentDelivery bool
		err := tx.DB().QueryRow(ctx, `
			SELECT event.id,
			       event.recipient_count,
			       EXISTS (
			           SELECT 1
			           FROM public.wechat_message_reminder_jobs job
			           WHERE job.event_type = $3
			             AND job.source_id = event.id
			             AND job.status IN ('pending', 'processing', 'sent')
			       )
			FROM public.daily_question_wechat_events event
			WHERE event.class_id = $1
			  AND event.assignment_date = $2::date
			  AND event.kind IN ('manual_student_reminder', 'automatic_student_reminder')
			ORDER BY event.created_at DESC, event.id DESC
			LIMIT 1
			FOR UPDATE`,
			input.ClassID,
			input.AssignmentDate,
			string(wechatreminder.EventDailyQuestion),
		).Scan(&existingID, &recipientCount, &hasActiveOrSentDelivery)
		if err == nil {
			if hasActiveOrSentDelivery {
				added, err := tx.wechatReminders.EnqueueDailyQuestionRecipients(
					ctx,
					tx.DB(),
					existingID,
					input.ClassID,
					input.AssignmentDate,
				)
				if err != nil {
					return err
				}
				if added > 0 {
					if _, err := tx.DB().Exec(ctx, `
						UPDATE public.daily_question_wechat_events
						SET recipient_count = recipient_count + $2
						WHERE id = $1`, existingID, added); err != nil {
						return err
					}
					recipientCount += added
				}
				result = dailyquestionapp.ReminderResult{
					ReminderID:     existingID,
					RecipientCount: recipientCount,
					Created:        false,
				}
				return nil
			}

			// Terminal jobs use the event ID as their semantic source. Remove
			// that source before retrying so a later binding or recovered queue
			// can receive a fresh job without duplicating a sent reminder.
			if _, err := tx.DB().Exec(ctx, `
				DELETE FROM public.daily_question_wechat_events
				WHERE id = $1`, existingID); err != nil {
				return err
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		if _, err := tx.DB().Exec(ctx, `
			INSERT INTO public.daily_question_wechat_events (
				id, kind, teacher_id, class_id, assignment_date, recipient_count, created_at
			)
			VALUES ($1, $2, $3, $4, $5::date, 0, $6)`,
			input.ReminderID,
			input.Kind,
			input.TeacherID,
			input.ClassID,
			input.AssignmentDate,
			input.Now,
		); err != nil {
			return err
		}

		recipientCount, err = tx.wechatReminders.EnqueueDailyQuestionRecipients(
			ctx,
			tx.DB(),
			input.ReminderID,
			input.ClassID,
			input.AssignmentDate,
		)
		if err != nil {
			return err
		}
		if recipientCount == 0 {
			_, err := tx.DB().Exec(ctx, `
				DELETE FROM public.daily_question_wechat_events
				WHERE id = $1`, input.ReminderID)
			return err
		}
		if _, err := tx.DB().Exec(ctx, `
			UPDATE public.daily_question_wechat_events
			SET recipient_count = $2
			WHERE id = $1`, input.ReminderID, recipientCount); err != nil {
			return err
		}
		result = dailyquestionapp.ReminderResult{
			ReminderID:     input.ReminderID,
			RecipientCount: recipientCount,
			Created:        true,
		}
		return nil
	})
	return result, found, err
}

// DispatchAutomaticReminders queues today's automatic reminders for every class
// with the persisted setting enabled. Per-class event uniqueness makes retries
// and multiple API instances safe.
func (r DailyQuestionRepository) DispatchAutomaticReminders(ctx context.Context, date, now time.Time) error {
	if !r.wechatReminders.Enabled() {
		return nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT class.id, class.teacher_id
		FROM public.daily_question_class_settings settings
		JOIN public.classes class ON class.id = settings.class_id
		WHERE settings.auto_reminder_enabled = true
		ORDER BY class.id`)
	if err != nil {
		return err
	}
	targets := make([]struct {
		classID   string
		teacherID string
	}, 0)
	for rows.Next() {
		var classID string
		var teacherID string
		if err := rows.Scan(&classID, &teacherID); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, struct {
			classID   string
			teacherID string
		}{classID: classID, teacherID: teacherID})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	var dispatchErr error
	for _, target := range targets {
		reminderID, err := newUUID()
		if err != nil {
			dispatchErr = errors.Join(dispatchErr, err)
			continue
		}
		if _, _, err := r.CreateClassReminder(ctx, dailyquestionapp.ClassReminderInput{
			ReminderID:     reminderID,
			TeacherID:      target.teacherID,
			ClassID:        target.classID,
			AssignmentDate: date,
			Kind:           dailyquestionapp.ReminderKindAutomaticStudent,
			Now:            now,
		}); err != nil {
			dispatchErr = errors.Join(dispatchErr, err)
		}
	}
	return dispatchErr
}

// EnsureUniformLowStockAlert queues a teacher-only WeChat event when the class
// has exactly one unlocked valid uniform question remaining from today onward.
func (r DailyQuestionRepository) EnsureUniformLowStockAlert(ctx context.Context, teacherID, classID string, date, now time.Time) error {
	if !r.wechatReminders.Enabled() {
		return nil
	}
	return withRepositoryTx(ctx, "daily question uniform low stock", r.Repository, func(base Repository) DailyQuestionRepository {
		return DailyQuestionRepository{Repository: base, wechatReminders: r.wechatReminders}
	}, func(tx DailyQuestionRepository) error {
		var ownedClassID string
		var strategy string
		if err := tx.DB().QueryRow(ctx, `
			SELECT class.id, coalesce(settings.strategy, 'personalized')
			FROM public.classes class
			LEFT JOIN public.daily_question_class_settings settings ON settings.class_id = class.id
			WHERE class.id = $1
			  AND class.teacher_id = $2
			FOR UPDATE OF class`, classID, teacherID).Scan(&ownedClassID, &strategy); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if strategy != dailyquestionapp.StrategyUniform {
			_, err := tx.DB().Exec(ctx, `
				DELETE FROM public.daily_question_wechat_events
				WHERE class_id = $1
				  AND kind = 'uniform_low_stock'`, classID)
			return err
		}
		return tx.enqueueUniformLowStockAlert(ctx, teacherID, classID, date, now)
	})
}

// DispatchUniformLowStockAlerts reconciles the one-question threshold as the
// Shanghai calendar advances, without relying on a teacher page visit.
func (r DailyQuestionRepository) DispatchUniformLowStockAlerts(ctx context.Context, date, now time.Time) error {
	if !r.wechatReminders.Enabled() {
		return nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT class.id, class.teacher_id
		FROM public.classes class
		JOIN public.daily_question_class_settings settings ON settings.class_id = class.id
		WHERE settings.strategy = 'uniform'
		ORDER BY class.id`)
	if err != nil {
		return err
	}
	targets := make([]struct {
		classID   string
		teacherID string
	}, 0)
	for rows.Next() {
		var classID string
		var teacherID string
		if err := rows.Scan(&classID, &teacherID); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, struct {
			classID   string
			teacherID string
		}{classID: classID, teacherID: teacherID})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	var dispatchErr error
	for _, target := range targets {
		if err := r.EnsureUniformLowStockAlert(ctx, target.teacherID, target.classID, date, now); err != nil {
			dispatchErr = errors.Join(dispatchErr, err)
		}
	}
	return dispatchErr
}

func (r DailyQuestionRepository) enqueueUniformLowStockAlert(
	ctx context.Context,
	teacherID string,
	classID string,
	date time.Time,
	now time.Time,
) error {
	rows, err := r.DB().Query(ctx, `
		SELECT selection.content_id
		FROM public.daily_question_class_selections selection
		JOIN public.contents content ON content.id = selection.content_id
		WHERE selection.class_id = $1
		  AND selection.assignment_date >= $2::date
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
		ORDER BY selection.assignment_date
		LIMIT 2`, classID, date)
	if err != nil {
		return err
	}
	defer rows.Close()
	remaining := make([]string, 0, 2)
	for rows.Next() {
		var contentID string
		if err := rows.Scan(&contentID); err != nil {
			return err
		}
		remaining = append(remaining, contentID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(remaining) != 1 {
		_, err := r.DB().Exec(ctx, `
			DELETE FROM public.daily_question_wechat_events
			WHERE class_id = $1
			  AND kind = 'uniform_low_stock'`, classID)
		return err
	}
	if _, err := r.DB().Exec(ctx, `
		DELETE FROM public.daily_question_wechat_events
		WHERE class_id = $1
		  AND kind = 'uniform_low_stock'
		  AND remaining_content_id <> $2`, classID, remaining[0]); err != nil {
		return err
	}

	var existingID string
	var hasActiveOrSentDelivery bool
	err = r.DB().QueryRow(ctx, `
		SELECT event.id,
		       EXISTS (
		           SELECT 1
		           FROM public.wechat_message_reminder_jobs job
		           WHERE job.event_type = $3
		             AND job.source_id = event.id
		             AND job.status IN ('pending', 'processing', 'sent')
		       )
		FROM public.daily_question_wechat_events event
		WHERE event.class_id = $1
		  AND event.remaining_content_id = $2
		  AND event.kind = 'uniform_low_stock'
		FOR UPDATE`,
		classID,
		remaining[0],
		string(wechatreminder.EventDailyQuestion),
	).Scan(&existingID, &hasActiveOrSentDelivery)
	if err == nil {
		if hasActiveOrSentDelivery {
			return nil
		}
		if _, err := r.DB().Exec(ctx, `
			DELETE FROM public.daily_question_wechat_events
			WHERE id = $1`, existingID); err != nil {
			return err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	eventID, err := newUUID()
	if err != nil {
		return err
	}
	var insertedID string
	err = r.DB().QueryRow(ctx, `
		INSERT INTO public.daily_question_wechat_events (
			id, kind, teacher_id, class_id, assignment_date, remaining_content_id, recipient_count, created_at
		)
		VALUES ($1, 'uniform_low_stock', $2, $3, $4::date, $5, 1, $6)
		RETURNING id`, eventID, teacherID, classID, date, remaining[0], now).Scan(&insertedID)
	if err != nil {
		return err
	}
	return r.wechatReminders.Enqueue(
		ctx,
		r.DB(),
		wechatreminder.EventDailyQuestion,
		insertedID,
		teacherID,
	)
}

const assignmentSelectSQL = `
	SELECT a.id,
	       a.student_id,
	       a.class_id,
	       a.assignment_date,
	       a.content_id,
	       a.target_concept_id,
	       target_node.name,
	       a.source,
	       a.selection_reason,
	       a.status,
	       a.first_attempt_id,
	       a.corrected_attempt_id,
	       a.first_result,
	       a.counts_toward_streak,
	       a.generation_token,
	       a.retry_count,
	       a.failure_code,
	       a.assigned_at,
	       a.opened_at,
	       a.completed_at,
	       a.updated_at,
	       CASE WHEN a.question_body IS NOT NULL THEN a.content_id ELSE content.id END,
	       coalesce(a.question_title, content.title),
	       coalesce(a.question_body, content.body),
	       coalesce(a.question_difficulty, content.difficulty),
	       coalesce(a.question_concept_ids, content.concept_ids),
	       coalesce(a.question_meta, content.meta),
	       coalesce(a.question_generated_by_student_id, content.generated_by_student_id)
	FROM public.daily_question_assignments a
	LEFT JOIN public.contents content
	  ON content.id = a.content_id
	 AND content.type = 'PROBLEM'::public.contenttype
	LEFT JOIN public.knowledge_nodes target_node ON target_node.id = a.target_concept_id`

func scanOptionalDailyAssignment(row pgx.Row) (dailyquestionapp.Assignment, bool, error) {
	assignment, err := scanDailyAssignment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return dailyquestionapp.Assignment{}, false, nil
	}
	return assignment, err == nil, err
}

func scanDailyAssignment(scanner rowScanner) (dailyquestionapp.Assignment, error) {
	var assignment dailyquestionapp.Assignment
	var classID pgtype.Text
	var contentID pgtype.Text
	var targetConceptID pgtype.Text
	var targetConceptName pgtype.Text
	var firstAttemptID pgtype.Text
	var correctedAttemptID pgtype.Text
	var firstResult pgtype.Text
	var generationToken pgtype.Text
	var failureCode pgtype.Text
	var openedAt pgtype.Timestamp
	var completedAt pgtype.Timestamp
	var questionID pgtype.Text
	var title pgtype.Text
	var body pgtype.Text
	var difficulty pgtype.Float8
	var conceptIDsRaw []byte
	var metaRaw []byte
	var generatedBy pgtype.Text
	if err := scanner.Scan(
		&assignment.ID,
		&assignment.StudentID,
		&classID,
		&assignment.AssignmentDate,
		&contentID,
		&targetConceptID,
		&targetConceptName,
		&assignment.Source,
		&assignment.SelectionReason,
		&assignment.Status,
		&firstAttemptID,
		&correctedAttemptID,
		&firstResult,
		&assignment.CountsTowardStreak,
		&generationToken,
		&assignment.RetryCount,
		&failureCode,
		&assignment.AssignedAt,
		&openedAt,
		&completedAt,
		&assignment.UpdatedAt,
		&questionID,
		&title,
		&body,
		&difficulty,
		&conceptIDsRaw,
		&metaRaw,
		&generatedBy,
	); err != nil {
		return dailyquestionapp.Assignment{}, err
	}
	assignment.ClassID = textPointer(classID)
	assignment.ContentID = textPointer(contentID)
	assignment.TargetConceptID = textPointer(targetConceptID)
	assignment.TargetConceptName = textPointer(targetConceptName)
	assignment.FirstAttemptID = textPointer(firstAttemptID)
	assignment.CorrectedAttemptID = textPointer(correctedAttemptID)
	assignment.FirstResult = textPointer(firstResult)
	assignment.GenerationToken = textPointer(generationToken)
	assignment.FailureCode = textPointer(failureCode)
	assignment.OpenedAt = timestampPointer(openedAt)
	assignment.CompletedAt = timestampPointer(completedAt)
	if !questionID.Valid {
		return assignment, nil
	}
	conceptIDs, err := decodeStringSlice(conceptIDsRaw)
	if err != nil {
		return dailyquestionapp.Assignment{}, fmt.Errorf("decode daily question concept ids: %w", err)
	}
	meta, err := decodeObjectMap(metaRaw)
	if err != nil {
		return dailyquestionapp.Assignment{}, fmt.Errorf("decode daily question meta: %w", err)
	}
	questionSource := exerciseapp.ExerciseSourceClass
	if generatedBy.Valid {
		questionSource = exerciseapp.ExerciseSourceAIGenerated
	}
	assignment.Question = &exerciseapp.ExerciseResponse{
		ID:                   questionID.String,
		Title:                title.String,
		Content:              body.String,
		Difficulty:           difficulty.Float64,
		Type:                 metaString(meta, "type", exerciseapp.QuestionTypeShortAnswer),
		Source:               questionSource,
		KnowledgePoints:      append([]string(nil), conceptIDs...),
		KnowledgePointNames:  metautil.StringSlice(meta, "knowledge_point_names"),
		HintsAvailable:       len(metautil.StringSlice(meta, "hints")) > 0,
		EstimatedTimeSeconds: metautil.IntDefault(meta, "estimated_time_seconds", 300),
		Options:              metautil.OptionalStringSlice(meta, "options"),
	}
	return assignment, nil
}

func scanOptionalClassSelection(row pgx.Row) (dailyquestionapp.ClassSelection, bool, error) {
	var selection dailyquestionapp.ClassSelection
	var targetConceptID pgtype.Text
	err := row.Scan(
		&selection.ClassID,
		&selection.AssignmentDate,
		&selection.ContentID,
		&targetConceptID,
		&selection.Source,
		&selection.SelectionReason,
		&selection.QuestionBody,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dailyquestionapp.ClassSelection{}, false, nil
	}
	if err != nil {
		return dailyquestionapp.ClassSelection{}, false, err
	}
	if targetConceptID.Valid {
		selection.TargetConceptID = targetConceptID.String
	}
	return selection, true, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func textPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return stringPtr(value.String)
}

func timestampPointer(value pgtype.Timestamp) *time.Time {
	if !value.Valid {
		return nil
	}
	copy := value.Time
	return &copy
}

func stringPtr(value string) *string {
	copy := value
	return &copy
}

func containsString(values []string, target string) bool {
	if target == "" {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func metaString(meta map[string]any, key, fallback string) string {
	value := strings.TrimSpace(metautil.String(meta, key))
	if value == "" {
		return fallback
	}
	return value
}

func percent(total, part int) float64 {
	if total <= 0 || part <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}
