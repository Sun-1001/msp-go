package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	progressapp "mathstudy/backend-go/internal/application/progress"
)

// ProgressRepository persists progress read models in PostgreSQL.
type ProgressRepository struct {
	Repository
}

// NewProgressRepository creates a PostgreSQL-backed progress repository.
func NewProgressRepository(db Querier) (ProgressRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return ProgressRepository{}, err
	}
	return ProgressRepository{Repository: base}, nil
}

// GetProfile returns progress counters from student_profiles.
func (r ProgressRepository) GetProfile(ctx context.Context, userID string) (progressapp.StudentProfile, bool, error) {
	var profile progressapp.StudentProfile
	var masteryRaw []byte
	err := r.DB().QueryRow(ctx, `
		SELECT total_exercises, correct_count, mastery_vector
		FROM public.student_profiles
		WHERE student_id = $1`,
		userID,
	).Scan(&profile.TotalExercises, &profile.CorrectCount, &masteryRaw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return progressapp.StudentProfile{}, false, nil
		}
		return progressapp.StudentProfile{}, false, err
	}
	mastery, err := decodeFloatMap(masteryRaw)
	if err != nil {
		return progressapp.StudentProfile{}, false, fmt.Errorf("decode mastery vector: %w", err)
	}
	profile.MasteryVector = mastery
	return profile, true, nil
}

// GetAttemptTotals derives total and correct attempt counts.
func (r ProgressRepository) GetAttemptTotals(ctx context.Context, userID string) (int, int, error) {
	var total int
	var correct int
	err := r.DB().QueryRow(ctx, `
		SELECT
			count(id)::int,
			coalesce(sum(CASE WHEN is_correct THEN 1 ELSE 0 END), 0)::int
		FROM public.content_attempts
		WHERE student_id = $1`,
		userID,
	).Scan(&total, &correct)
	return total, correct, err
}

// SumStudySeconds sums attempt time, optionally from a started_at lower bound.
func (r ProgressRepository) SumStudySeconds(ctx context.Context, userID string, since *time.Time) (int, error) {
	var total int
	var err error
	if since == nil {
		err = r.DB().QueryRow(ctx, `
			SELECT coalesce(sum(time_spent_seconds), 0)::int
			FROM public.content_attempts
			WHERE student_id = $1`,
			userID,
		).Scan(&total)
	} else {
		err = r.DB().QueryRow(ctx, `
			SELECT coalesce(sum(time_spent_seconds), 0)::int
			FROM public.content_attempts
			WHERE student_id = $1 AND started_at >= $2`,
			userID,
			*since,
		).Scan(&total)
	}
	return total, err
}

// CountAttemptsStartedSince counts attempts started after the given instant.
func (r ProgressRepository) CountAttemptsStartedSince(ctx context.Context, userID string, since time.Time) (int, error) {
	var count int
	err := r.DB().QueryRow(ctx, `
		SELECT count(id)::int
		FROM public.content_attempts
		WHERE student_id = $1 AND started_at >= $2`,
		userID,
		since,
	).Scan(&count)
	return count, err
}

// LatestAttemptStartedAt returns the latest started_at value for one student.
func (r ProgressRepository) LatestAttemptStartedAt(ctx context.Context, userID string) (*time.Time, error) {
	var startedAt time.Time
	err := r.DB().QueryRow(ctx, `
		SELECT started_at
		FROM public.content_attempts
		WHERE student_id = $1
		ORDER BY started_at DESC
		LIMIT 1`,
		userID,
	).Scan(&startedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &startedAt, nil
}

// ListSubmittedAttemptDays returns active submitted days, newest first.
func (r ProgressRepository) ListSubmittedAttemptDays(ctx context.Context, userID string, limit int) ([]time.Time, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT date_trunc('day', submitted_at) AS day
		FROM public.content_attempts
		WHERE student_id = $1 AND submitted_at IS NOT NULL
		GROUP BY day
		ORDER BY day DESC
		LIMIT $2`,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	days := []time.Time{}
	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		days = append(days, day)
	}
	return days, rows.Err()
}

// ListMasteryStates returns DKT mastery states for a student.
func (r ProgressRepository) ListMasteryStates(ctx context.Context, userID string, conceptIDs []string) ([]progressapp.MasteryState, error) {
	query := `
		SELECT concept_id, mastery_prob, confidence, attempt_count, last_attempt_at
		FROM public.student_concept_dkt_states
		WHERE student_id = $1`
	args := []any{userID}
	if len(conceptIDs) > 0 {
		query += ` AND concept_id = ANY($2::varchar[])`
		args = append(args, conceptIDs)
	}
	query += ` ORDER BY concept_id`

	rows, err := r.DB().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := []progressapp.MasteryState{}
	for rows.Next() {
		var state progressapp.MasteryState
		var lastAttempt pgtype.Timestamp
		if err := rows.Scan(
			&state.ConceptID,
			&state.Mastery,
			&state.Confidence,
			&state.AttemptCount,
			&lastAttempt,
		); err != nil {
			return nil, err
		}
		if lastAttempt.Valid {
			value := lastAttempt.Time
			state.LastAttemptAt = &value
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

// ListKnowledgeNodes returns knowledge graph nodes with optional filters.
func (r ProgressRepository) ListKnowledgeNodes(ctx context.Context, filter progressapp.KnowledgeNodeFilter) ([]progressapp.KnowledgeNode, error) {
	nodeType := progressNodeTypeToDB(filter.NodeType)
	rows, err := r.DB().Query(ctx, `
		SELECT id, name, node_type::text, description, chapter, difficulty, created_at
		FROM public.knowledge_nodes
		WHERE
			($1 = '' OR chapter = $1) AND
			($2 = '' OR node_type::text = $2) AND
			(
				$3 = '' OR
				name ILIKE '%' || $3 || '%' OR
				name_en ILIKE '%' || $3 || '%' OR
				description ILIKE '%' || $3 || '%'
			)
		ORDER BY created_at`,
		filter.Chapter,
		nodeType,
		filter.Search,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := []progressapp.KnowledgeNode{}
	for rows.Next() {
		node, err := scanKnowledgeNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

// ListKnowledgeRelations returns all knowledge graph relations.
func (r ProgressRepository) ListKnowledgeRelations(ctx context.Context) ([]progressapp.KnowledgeRelation, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT id, source_id, target_id, relation_type::text, created_at
		FROM public.knowledge_relations
		ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	relations := []progressapp.KnowledgeRelation{}
	for rows.Next() {
		var relation progressapp.KnowledgeRelation
		if err := rows.Scan(
			&relation.ID,
			&relation.SourceID,
			&relation.TargetID,
			&relation.RelationType,
			&relation.CreatedAt,
		); err != nil {
			return nil, err
		}
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

// ListLearningStatsByDay returns submitted attempt aggregates grouped by day.
func (r ProgressRepository) ListLearningStatsByDay(ctx context.Context, userID string, start time.Time, end time.Time) ([]progressapp.PeriodStat, error) {
	return r.listLearningStats(ctx, userID, start, end, "day")
}

// ListLearningStatsByWeek returns submitted attempt aggregates grouped by week.
func (r ProgressRepository) ListLearningStatsByWeek(ctx context.Context, userID string, start time.Time, end time.Time) ([]progressapp.PeriodStat, error) {
	return r.listLearningStats(ctx, userID, start, end, "week")
}

// CountErrorsByType returns diagnosis error counts in a submitted-at range.
func (r ProgressRepository) CountErrorsByType(ctx context.Context, userID string, start time.Time, end time.Time) (map[string]int, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT coalesce(dr.error_type::text, 'UNKNOWN') AS error_type, count(dr.id)::int
		FROM public.diagnosis_reports dr
		JOIN public.content_attempts ca ON dr.attempt_id = ca.id
		WHERE
			ca.student_id = $1 AND
			ca.submitted_at IS NOT NULL AND
			ca.submitted_at >= $2 AND
			ca.submitted_at <= $3
		GROUP BY error_type`,
		userID,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		counts[key] += count
	}
	return counts, rows.Err()
}

// ListClassStudentIDs returns students in the current user's class.
func (r ProgressRepository) ListClassStudentIDs(ctx context.Context, userID string) ([]string, bool, error) {
	var classID string
	err := r.DB().QueryRow(ctx, `
		SELECT class_id
		FROM public.class_enrollments
		WHERE student_id = $1`,
		userID,
	).Scan(&classID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	rows, err := r.DB().Query(ctx, `
		SELECT student_id
		FROM public.class_enrollments
		WHERE class_id = $1`,
		classID,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	studentIDs := []string{}
	for rows.Next() {
		var studentID string
		if err := rows.Scan(&studentID); err != nil {
			return nil, false, err
		}
		studentIDs = append(studentIDs, studentID)
	}
	return studentIDs, true, rows.Err()
}

// AttemptStatsForStudents returns ranking stats for a group of students.
func (r ProgressRepository) AttemptStatsForStudents(ctx context.Context, studentIDs []string) (map[string]progressapp.StudentAttemptStats, error) {
	stats := map[string]progressapp.StudentAttemptStats{}
	if len(studentIDs) == 0 {
		return stats, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT
			student_id,
			coalesce(sum(time_spent_seconds), 0)::int AS total_seconds,
			count(id)::int AS attempt_count
		FROM public.content_attempts
		WHERE student_id = ANY($1::varchar[])
		GROUP BY student_id`,
		studentIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var studentID string
		var stat progressapp.StudentAttemptStats
		if err := rows.Scan(&studentID, &stat.StudySeconds, &stat.AttemptCount); err != nil {
			return nil, err
		}
		stats[studentID] = stat
	}
	return stats, rows.Err()
}

// DistinctChapters returns sorted non-empty chapter names.
func (r ProgressRepository) DistinctChapters(ctx context.Context) ([]string, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT DISTINCT chapter
		FROM public.knowledge_nodes
		WHERE chapter IS NOT NULL AND chapter <> ''
		ORDER BY chapter`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chapters := []string{}
	for rows.Next() {
		var chapter string
		if err := rows.Scan(&chapter); err != nil {
			return nil, err
		}
		chapters = append(chapters, chapter)
	}
	return chapters, rows.Err()
}

func (r ProgressRepository) listLearningStats(ctx context.Context, userID string, start time.Time, end time.Time, interval string) ([]progressapp.PeriodStat, error) {
	truncUnit := "day"
	if interval == "week" {
		truncUnit = "week"
	}
	rows, err := r.DB().Query(ctx, `
		SELECT
			date_trunc('`+truncUnit+`', submitted_at) AS period,
			count(id)::int AS total,
			coalesce(sum(CASE WHEN is_correct THEN 1 ELSE 0 END), 0)::int AS correct,
			coalesce(sum(time_spent_seconds), 0)::int AS time_spent
		FROM public.content_attempts
		WHERE
			student_id = $1 AND
			submitted_at IS NOT NULL AND
			submitted_at >= $2 AND
			submitted_at <= $3
		GROUP BY period
		ORDER BY period`,
		userID,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []progressapp.PeriodStat{}
	for rows.Next() {
		var stat progressapp.PeriodStat
		if err := rows.Scan(&stat.Date, &stat.Exercises, &stat.CorrectExercises, &stat.StudySeconds); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	return stats, rows.Err()
}

func scanKnowledgeNode(rows pgx.Rows) (progressapp.KnowledgeNode, error) {
	var node progressapp.KnowledgeNode
	var chapter pgtype.Text
	if err := rows.Scan(
		&node.ID,
		&node.Name,
		&node.NodeType,
		&node.Description,
		&chapter,
		&node.Difficulty,
		&node.CreatedAt,
	); err != nil {
		return progressapp.KnowledgeNode{}, err
	}
	if chapter.Valid {
		value := chapter.String
		node.Chapter = &value
	}
	return node, nil
}

func progressNodeTypeToDB(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "concept":
		return "CONCEPT"
	case "theorem":
		return "THEOREM"
	case "method":
		return "METHOD"
	default:
		return ""
	}
}
