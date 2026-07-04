package mistake

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"mathstudy/backend-go/internal/platform/maputil"
	"mathstudy/backend-go/internal/platform/metautil"
	"mathstudy/backend-go/internal/platform/ptrutil"
	"mathstudy/backend-go/internal/platform/sliceutil"
	"mathstudy/backend-go/internal/platform/timefmt"
)

var (
	// ErrNotFound is returned when a requested mistake record does not exist for the user.
	ErrNotFound = errors.New("mistake not found")
	// ErrProfileNotFound is returned when a mistake write needs a missing student profile.
	ErrProfileNotFound = errors.New("student profile not found")
)

// Repository is the persistence surface required by mistake book use cases.
type Repository interface {
	ListMistakes(context.Context, string, ListFilter) ([]MistakeRow, error)
	ListMistakePage(context.Context, string, ListQuery) ([]MistakeListRow, int, error)
	GetMistakeByAttempt(context.Context, string, string) (MistakeRow, bool, error)
	GetAttemptContent(context.Context, string, string) (AttemptContent, bool, error)
	ListAttemptHistory(context.Context, string, string, string) ([]MistakeHistory, error)
	GetProfile(context.Context, string) (StudentProfile, bool, error)
	ErrorCountsByContent(context.Context, string) (map[string]int, error)
	CountSubmittedAttempts(context.Context, string, *time.Time, *time.Time) (int, error)
	UpdateProfileMastery(context.Context, string, map[string]float64, time.Time) (bool, error)
	DeleteAttempt(context.Context, string, string) (bool, error)
}

// ListFilter stores database-level mistake filters.
type ListFilter struct {
	ErrorType     string
	ConceptID     string
	DifficultyMin float64
	DifficultyMax float64
	DateFrom      *time.Time
	DateTo        *time.Time
}

// ListQuery stores the full /mistakes list query.
type ListQuery struct {
	Page          int
	PageSize      int
	ErrorType     string
	ConceptID     string
	DifficultyMin float64
	DifficultyMax float64
	DateFrom      *time.Time
	DateTo        *time.Time
	MasteryStatus string
	SortBy        string
	SortOrder     string
}

// MistakeRow combines an attempt, diagnosis, and content record.
type MistakeRow struct {
	Attempt   Attempt
	Content   Content
	Diagnosis Diagnosis
}

// MistakeListRow stores one SQL-paginated mistake row with list aggregates.
type MistakeListRow struct {
	Row        MistakeRow
	AvgMastery float64
	ErrorCount int
}

// AttemptContent combines an attempt and content row for write use cases.
type AttemptContent struct {
	Attempt Attempt
	Content Content
}

// Attempt stores student answer data.
type Attempt struct {
	ID               string
	ContentID        string
	StudentAnswer    string
	StudentSteps     []string
	IsCorrect        bool
	Score            float64
	SubmittedAt      *time.Time
	TimeSpentSeconds int
}

// Content stores exercise-like content fields used by the mistake book.
type Content struct {
	ID         string
	Type       string
	Title      string
	Body       string
	Difficulty float64
	ConceptIDs []string
	Meta       map[string]any
}

// Diagnosis stores diagnostic metadata for a mistake.
type Diagnosis struct {
	ErrorType         *string
	ErrorSubtype      string
	Severity          string
	Explanation       string
	Suggestion        string
	RelatedConceptIDs []string
	ErrorStepIndex    *int
}

// StudentProfile stores mastery data used by the mistake book.
type StudentProfile struct {
	MasteryVector map[string]float64
}

// MistakeListResponse is the Python-compatible GET /mistakes response.
type MistakeListResponse struct {
	Items      []MistakeItem     `json:"items"`
	Pagination PaginationInfo    `json:"pagination"`
	Statistics MistakeStatistics `json:"statistics"`
}

// MistakeItem stores one list row.
type MistakeItem struct {
	ID             string           `json:"id"`
	Exercise       MistakeExercise  `json:"exercise"`
	Attempt        MistakeAttempt   `json:"attempt"`
	Diagnosis      MistakeDiagnosis `json:"diagnosis"`
	Mastery        MistakeMastery   `json:"mastery"`
	ErrorCount     int              `json:"error_count"`
	LastReviewedAt *string          `json:"last_reviewed_at"`
}

// MistakeExercise stores exercise summary data for a mistake row.
type MistakeExercise struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Difficulty      float64  `json:"difficulty"`
	KnowledgePoints []string `json:"knowledge_points"`
}

// MistakeAttempt stores answer summary data for a mistake row.
type MistakeAttempt struct {
	StudentAnswer    string  `json:"student_answer"`
	CorrectAnswer    string  `json:"correct_answer"`
	IsCorrect        bool    `json:"is_correct"`
	Score            float64 `json:"score"`
	SubmittedAt      *string `json:"submitted_at"`
	TimeSpentSeconds int     `json:"time_spent_seconds"`
}

// MistakeDiagnosis stores diagnosis summary data.
type MistakeDiagnosis struct {
	ErrorType       *string  `json:"error_type"`
	ErrorSubtype    string   `json:"error_subtype"`
	Severity        string   `json:"severity"`
	Explanation     string   `json:"explanation"`
	Suggestion      string   `json:"suggestion"`
	RelatedConcepts []string `json:"related_concepts"`
}

// MistakeMastery stores mastery state for a mistake row.
type MistakeMastery struct {
	Current  float64 `json:"current"`
	Previous float64 `json:"previous"`
	Trend    string  `json:"trend"`
}

// PaginationInfo stores page metadata.
type PaginationInfo struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// MistakeStatistics stores list-level summary data.
type MistakeStatistics struct {
	TotalMistakes int     `json:"total_mistakes"`
	WeakConcepts  int     `json:"weak_concepts"`
	AvgMastery    float64 `json:"avg_mastery"`
}

// StatisticsResponse is the Python-compatible GET /mistakes/statistics response.
type StatisticsResponse struct {
	Overview              StatisticsOverview               `json:"overview"`
	ErrorTypeDistribution map[string]ErrorTypeDistribution `json:"error_type_distribution"`
	ConceptWeakness       []ConceptWeakness                `json:"concept_weakness"`
}

// StatisticsOverview stores mistake summary counters.
type StatisticsOverview struct {
	TotalMistakes  int     `json:"total_mistakes"`
	TotalExercises int     `json:"total_exercises"`
	MistakeRate    float64 `json:"mistake_rate"`
	AvgMastery     float64 `json:"avg_mastery"`
}

// ErrorTypeDistribution stores an error type bucket.
type ErrorTypeDistribution struct {
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
	Label      string  `json:"label"`
}

// ConceptWeakness stores mistake count and mastery for one concept.
type ConceptWeakness struct {
	ConceptID      string  `json:"concept_id"`
	ConceptName    string  `json:"concept_name"`
	MistakeCount   int     `json:"mistake_count"`
	Mastery        float64 `json:"mastery"`
	RecentMistakes int     `json:"recent_mistakes"`
}

// DetailResponse is the Python-compatible GET /mistakes/{attempt_id} response.
type DetailResponse struct {
	AttemptID string                 `json:"attempt_id"`
	Exercise  MistakeDetailExercise  `json:"exercise"`
	Attempt   MistakeDetailAttempt   `json:"attempt"`
	Diagnosis MistakeDetailDiagnosis `json:"diagnosis"`
	Solution  MistakeSolution        `json:"solution"`
	History   []MistakeHistory       `json:"history"`
}

// MistakeDetailExercise stores detailed exercise data.
type MistakeDetailExercise struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Difficulty      float64  `json:"difficulty"`
	KnowledgePoints []string `json:"knowledge_points"`
	Hints           []string `json:"hints"`
}

// MistakeDetailAttempt stores detailed answer data.
type MistakeDetailAttempt struct {
	StudentAnswer    string   `json:"student_answer"`
	StudentSteps     []string `json:"student_steps"`
	CorrectAnswer    string   `json:"correct_answer"`
	SubmittedAt      *string  `json:"submitted_at"`
	TimeSpentSeconds int      `json:"time_spent_seconds"`
}

// MistakeDetailDiagnosis stores detailed diagnostic data.
type MistakeDetailDiagnosis struct {
	ErrorType       *string  `json:"error_type"`
	ErrorStepIndex  *int     `json:"error_step_index"`
	Explanation     string   `json:"explanation"`
	Suggestion      string   `json:"suggestion"`
	RelatedConcepts []string `json:"related_concepts"`
}

// MistakeSolution stores cached solution data.
type MistakeSolution struct {
	Answer string   `json:"answer"`
	Steps  []string `json:"steps"`
	Source string   `json:"source"`
}

// MistakeHistory stores prior attempts for the same content.
type MistakeHistory struct {
	AttemptID   string  `json:"attempt_id"`
	SubmittedAt *string `json:"submitted_at"`
	IsCorrect   bool    `json:"is_correct"`
	Score       float64 `json:"score"`
}

// MarkAsMasteredResponse is the Python-compatible POST /mistakes/{attempt_id}/master response.
type MarkAsMasteredResponse struct {
	Success       bool               `json:"success"`
	MasteredAt    string             `json:"mastered_at,omitempty"`
	MasteryUpdate map[string]float64 `json:"mastery_update,omitempty"`
}

// ReviewExerciseResponse is the Python-compatible GET /mistakes/review/next response.
type ReviewExerciseResponse struct {
	Exercise ReviewExercise `json:"exercise"`
	Context  ReviewContext  `json:"context"`
}

// ReviewExercise stores recommended exercise data.
type ReviewExercise struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Difficulty      float64  `json:"difficulty"`
	Type            string   `json:"type"`
	KnowledgePoints []string `json:"knowledge_points"`
	HintsAvailable  bool     `json:"hints_available"`
}

// ReviewContext stores context for the recommended review item.
type ReviewContext struct {
	IsReview          bool    `json:"is_review"`
	OriginalAttemptID string  `json:"original_attempt_id"`
	PreviousErrorType *string `json:"previous_error_type"`
	MasteryBefore     float64 `json:"mastery_before"`
	ErrorCount        int     `json:"error_count"`
}

// DeleteResponse is the Python-compatible DELETE /mistakes/{attempt_id} response.
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Service implements mistake book use cases.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService creates a mistake book service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("mistake repository is nil")
	}
	return &Service{repo: repo, now: time.Now}, nil
}

// GetMistakes returns paginated mistakes with filtering and sorting.
func (s *Service) GetMistakes(ctx context.Context, userID string, query ListQuery) (MistakeListResponse, error) {
	query = normalizeListQuery(query)
	rows, total, err := s.repo.ListMistakePage(ctx, userID, query)
	if err != nil {
		return MistakeListResponse{}, err
	}
	profile, _, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return MistakeListResponse{}, err
	}
	mastery := maputil.CloneFloatMap(profile.MasteryVector)

	responseItems := make([]MistakeItem, 0, len(rows))
	for _, row := range rows {
		responseItems = append(responseItems, toMistakeItem(listItemData{
			row:        row.Row,
			avgMastery: row.AvgMastery,
			errorCount: row.ErrorCount,
		}))
	}

	return MistakeListResponse{
		Items: responseItems,
		Pagination: PaginationInfo{
			Page:       query.Page,
			PageSize:   query.PageSize,
			Total:      total,
			TotalPages: totalPages(total, query.PageSize),
		},
		Statistics: MistakeStatistics{
			TotalMistakes: total,
			WeakConcepts:  countWeakConcepts(mastery),
			AvgMastery:    averageFloatMap(mastery),
		},
	}, nil
}

// GetStatistics returns mistake statistics for a time range.
func (s *Service) GetStatistics(ctx context.Context, userID string, timeRange string) (StatisticsResponse, error) {
	start, end := s.timeRange(timeRange)
	rows, err := s.repo.ListMistakes(ctx, userID, ListFilter{DifficultyMin: 0, DifficultyMax: 1, DateFrom: start, DateTo: end})
	if err != nil {
		return StatisticsResponse{}, err
	}
	profile, _, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return StatisticsResponse{}, err
	}
	mastery := maputil.CloneFloatMap(profile.MasteryVector)
	totalExercises, err := s.repo.CountSubmittedAttempts(ctx, userID, start, end)
	if err != nil {
		return StatisticsResponse{}, err
	}

	errorCounts := map[string]int{}
	conceptMistakes := map[string]int{}
	for _, row := range rows {
		if row.Diagnosis.ErrorType != nil {
			errorCounts[*row.Diagnosis.ErrorType]++
		}
		for _, conceptID := range row.Content.ConceptIDs {
			conceptMistakes[conceptID]++
		}
	}

	totalMistakes := len(rows)
	return StatisticsResponse{
		Overview: StatisticsOverview{
			TotalMistakes:  totalMistakes,
			TotalExercises: totalExercises,
			MistakeRate:    round1(percent(totalExercises, totalMistakes)),
			AvgMastery:     round2(averageFloatMap(mastery)),
		},
		ErrorTypeDistribution: buildErrorTypeDistribution(errorCounts, totalMistakes),
		ConceptWeakness:       buildConceptWeakness(conceptMistakes, mastery),
	}, nil
}

// GetMistakeDetail returns detailed data for a mistake.
func (s *Service) GetMistakeDetail(ctx context.Context, userID string, attemptID string) (DetailResponse, error) {
	row, ok, err := s.repo.GetMistakeByAttempt(ctx, userID, attemptID)
	if err != nil {
		return DetailResponse{}, err
	}
	if !ok {
		return DetailResponse{}, ErrNotFound
	}
	history, err := s.repo.ListAttemptHistory(ctx, userID, row.Content.ID, attemptID)
	if err != nil {
		return DetailResponse{}, err
	}
	return toDetailResponse(row, history), nil
}

// MarkAsMastered raises the related concept mastery values in the student profile.
func (s *Service) MarkAsMastered(ctx context.Context, userID string, attemptID string) (MarkAsMasteredResponse, error) {
	attemptContent, ok, err := s.repo.GetAttemptContent(ctx, userID, attemptID)
	if err != nil {
		return MarkAsMasteredResponse{}, err
	}
	if !ok {
		return MarkAsMasteredResponse{}, ErrNotFound
	}
	profile, ok, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return MarkAsMasteredResponse{}, err
	}
	if !ok {
		return MarkAsMasteredResponse{}, ErrProfileNotFound
	}

	mastery := maputil.CloneFloatMap(profile.MasteryVector)
	update := map[string]float64{}
	for _, conceptID := range attemptContent.Content.ConceptIDs {
		current := mastery[conceptID]
		if current == 0 {
			current = 0.5
		}
		next := math.Min(1.0, math.Max(0.8, current+0.2))
		mastery[conceptID] = next
		update[conceptID] = next
	}

	now := s.now()
	updated, err := s.repo.UpdateProfileMastery(ctx, userID, mastery, now)
	if err != nil {
		return MarkAsMasteredResponse{}, err
	}
	if !updated {
		return MarkAsMasteredResponse{}, ErrProfileNotFound
	}
	return MarkAsMasteredResponse{
		Success:       true,
		MasteredAt:    timefmt.DateTimeMicros(now),
		MasteryUpdate: update,
	}, nil
}

// DeleteMistake deletes the attempt row; diagnosis reports are removed by database cascade.
func (s *Service) DeleteMistake(ctx context.Context, userID string, attemptID string) (DeleteResponse, error) {
	deleted, err := s.repo.DeleteAttempt(ctx, userID, attemptID)
	if err != nil {
		return DeleteResponse{}, err
	}
	if !deleted {
		return DeleteResponse{}, ErrNotFound
	}
	return DeleteResponse{Success: true, Message: "错题记录已删除"}, nil
}

// GetReviewExercise returns the highest-priority review candidate.
func (s *Service) GetReviewExercise(ctx context.Context, userID string, focusConcept string, focusErrorType string) (ReviewExerciseResponse, error) {
	profile, _, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return ReviewExerciseResponse{}, err
	}
	mastery := maputil.CloneFloatMap(profile.MasteryVector)
	rows, err := s.repo.ListMistakes(ctx, userID, ListFilter{
		ErrorType:     focusErrorType,
		ConceptID:     focusConcept,
		DifficultyMin: 0,
		DifficultyMax: 1,
	})
	if err != nil {
		return ReviewExerciseResponse{}, err
	}
	errorCounts, err := s.repo.ErrorCountsByContent(ctx, userID)
	if err != nil {
		return ReviewExerciseResponse{}, err
	}

	candidates := make([]reviewCandidate, 0, len(rows))
	for _, row := range rows {
		avgMastery := averageMastery(row.Content.ConceptIDs, mastery)
		errorCount := errorCounts[row.Content.ID]
		if errorCount == 0 {
			errorCount = 1
		}
		if avgMastery >= 0.5 || errorCount < 2 {
			continue
		}
		candidates = append(candidates, reviewCandidate{
			row:        row,
			avgMastery: avgMastery,
			errorCount: errorCount,
			priority:   (1 - avgMastery) * float64(errorCount),
		})
	}
	if len(candidates) == 0 {
		return ReviewExerciseResponse{}, ErrNotFound
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].priority == candidates[j].priority {
			return compareOptionalTimeDesc(candidates[i].row.Attempt.SubmittedAt, candidates[j].row.Attempt.SubmittedAt)
		}
		return candidates[i].priority > candidates[j].priority
	})
	return toReviewResponse(candidates[0]), nil
}

func normalizeListQuery(query ListQuery) ListQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	if query.DifficultyMax == 0 {
		query.DifficultyMax = 1
	}
	if query.DifficultyMin < 0 {
		query.DifficultyMin = 0
	}
	if query.DifficultyMax > 1 {
		query.DifficultyMax = 1
	}
	if query.DifficultyMin > query.DifficultyMax {
		query.DifficultyMin, query.DifficultyMax = query.DifficultyMax, query.DifficultyMin
	}
	if strings.TrimSpace(query.MasteryStatus) == "" {
		query.MasteryStatus = "all"
	}
	if strings.TrimSpace(query.SortBy) == "" {
		query.SortBy = "time"
	}
	if strings.TrimSpace(query.SortOrder) == "" {
		query.SortOrder = "desc"
	}
	return query
}

func (s *Service) timeRange(value string) (*time.Time, *time.Time) {
	now := s.now()
	var start *time.Time
	switch value {
	case "week":
		value := now.AddDate(0, 0, -7)
		start = &value
	case "semester":
		value := now.AddDate(0, 0, -120)
		start = &value
	case "all":
		return nil, nil
	default:
		value := now.AddDate(0, 0, -30)
		start = &value
	}
	return start, &now
}

func toMistakeItem(item listItemData) MistakeItem {
	row := item.row
	return MistakeItem{
		ID: row.Attempt.ID,
		Exercise: MistakeExercise{
			ID:              row.Content.ID,
			Title:           nonEmpty(row.Content.Title, "无标题"),
			Content:         row.Content.Body,
			Difficulty:      row.Content.Difficulty,
			KnowledgePoints: sliceutil.CloneStrings(row.Content.ConceptIDs),
		},
		Attempt: MistakeAttempt{
			StudentAnswer:    row.Attempt.StudentAnswer,
			CorrectAnswer:    metautil.String(row.Content.Meta, "answer"),
			IsCorrect:        row.Attempt.IsCorrect,
			Score:            row.Attempt.Score,
			SubmittedAt:      timefmt.OptionalDateTimeMicros(row.Attempt.SubmittedAt),
			TimeSpentSeconds: row.Attempt.TimeSpentSeconds,
		},
		Diagnosis: MistakeDiagnosis{
			ErrorType:       ptrutil.Clone(row.Diagnosis.ErrorType),
			ErrorSubtype:    row.Diagnosis.ErrorSubtype,
			Severity:        row.Diagnosis.Severity,
			Explanation:     row.Diagnosis.Explanation,
			Suggestion:      row.Diagnosis.Suggestion,
			RelatedConcepts: sliceutil.CloneStrings(row.Diagnosis.RelatedConceptIDs),
		},
		Mastery: MistakeMastery{
			Current:  item.avgMastery,
			Previous: item.avgMastery,
			Trend:    masteryTrend(item.avgMastery),
		},
		ErrorCount: item.errorCount,
	}
}

func toDetailResponse(row MistakeRow, history []MistakeHistory) DetailResponse {
	solutionSteps := metautil.StringSlice(row.Content.Meta, "solution_steps")
	source := "unavailable"
	if len(solutionSteps) > 0 {
		source = "cached"
	}
	return DetailResponse{
		AttemptID: row.Attempt.ID,
		Exercise: MistakeDetailExercise{
			ID:              row.Content.ID,
			Title:           nonEmpty(row.Content.Title, "无标题"),
			Content:         row.Content.Body,
			Difficulty:      row.Content.Difficulty,
			KnowledgePoints: sliceutil.CloneStrings(row.Content.ConceptIDs),
			Hints:           metautil.StringSlice(row.Content.Meta, "hints"),
		},
		Attempt: MistakeDetailAttempt{
			StudentAnswer:    row.Attempt.StudentAnswer,
			StudentSteps:     sliceutil.CloneStrings(row.Attempt.StudentSteps),
			CorrectAnswer:    metautil.String(row.Content.Meta, "answer"),
			SubmittedAt:      timefmt.OptionalDateTimeMicros(row.Attempt.SubmittedAt),
			TimeSpentSeconds: row.Attempt.TimeSpentSeconds,
		},
		Diagnosis: MistakeDetailDiagnosis{
			ErrorType:       ptrutil.Clone(row.Diagnosis.ErrorType),
			ErrorStepIndex:  ptrutil.Clone(row.Diagnosis.ErrorStepIndex),
			Explanation:     row.Diagnosis.Explanation,
			Suggestion:      row.Diagnosis.Suggestion,
			RelatedConcepts: sliceutil.CloneStrings(row.Diagnosis.RelatedConceptIDs),
		},
		Solution: MistakeSolution{
			Answer: metautil.String(row.Content.Meta, "answer"),
			Steps:  solutionSteps,
			Source: source,
		},
		History: history,
	}
}

func toReviewResponse(candidate reviewCandidate) ReviewExerciseResponse {
	row := candidate.row
	return ReviewExerciseResponse{
		Exercise: ReviewExercise{
			ID:              row.Content.ID,
			Title:           nonEmpty(row.Content.Title, "无标题"),
			Content:         row.Content.Body,
			Difficulty:      row.Content.Difficulty,
			Type:            contentTypeValue(row.Content.Type),
			KnowledgePoints: sliceutil.CloneStrings(row.Content.ConceptIDs),
			HintsAvailable:  len(metautil.StringSlice(row.Content.Meta, "hints")) > 0,
		},
		Context: ReviewContext{
			IsReview:          true,
			OriginalAttemptID: row.Attempt.ID,
			PreviousErrorType: ptrutil.Clone(row.Diagnosis.ErrorType),
			MasteryBefore:     candidate.avgMastery,
			ErrorCount:        candidate.errorCount,
		},
	}
}

func sortListItems(items []listItemData, sortBy string, sortOrder string) {
	descending := strings.EqualFold(sortOrder, "desc")
	sort.SliceStable(items, func(i, j int) bool {
		cmp := 0
		switch sortBy {
		case "error_count":
			cmp = compareInt(items[i].errorCount, items[j].errorCount)
		case "mastery":
			cmp = compareFloat(items[i].avgMastery, items[j].avgMastery)
		default:
			cmp = compareOptionalTime(items[i].row.Attempt.SubmittedAt, items[j].row.Attempt.SubmittedAt)
		}
		if cmp == 0 {
			cmp = strings.Compare(items[i].row.Attempt.ID, items[j].row.Attempt.ID)
		}
		if descending {
			cmp = -cmp
		}
		return cmp < 0
	})
}

func matchesMasteryStatus(avgMastery float64, status string) bool {
	switch status {
	case "weak":
		return avgMastery < 0.4
	case "improving":
		return avgMastery >= 0.4 && avgMastery < 0.7
	case "mastered":
		return avgMastery >= 0.7
	default:
		return true
	}
}

func buildErrorTypeDistribution(counts map[string]int, total int) map[string]ErrorTypeDistribution {
	labels := map[string]string{
		"conceptual":  "概念性错误",
		"procedural":  "过程性错误",
		"logical":     "逻辑错误",
		"symbolic":    "符号错误",
		"calculation": "计算错误",
	}
	distribution := map[string]ErrorTypeDistribution{}
	for key, count := range counts {
		percentage := 0.0
		if total > 0 {
			percentage = round1(float64(count) / float64(total) * 100)
		}
		label := labels[key]
		if label == "" {
			label = "未知错误"
		}
		distribution[key] = ErrorTypeDistribution{Count: count, Percentage: percentage, Label: label}
	}
	return distribution
}

func buildConceptWeakness(counts map[string]int, mastery map[string]float64) []ConceptWeakness {
	items := make([]ConceptWeakness, 0, len(counts))
	for conceptID, count := range counts {
		items = append(items, ConceptWeakness{
			ConceptID:      conceptID,
			ConceptName:    conceptID,
			MistakeCount:   count,
			Mastery:        masteryValue(conceptID, mastery),
			RecentMistakes: count,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].MistakeCount == items[j].MistakeCount {
			return items[i].ConceptID < items[j].ConceptID
		}
		return items[i].MistakeCount > items[j].MistakeCount
	})
	if len(items) > 10 {
		return items[:10]
	}
	return items
}

func averageMastery(conceptIDs []string, mastery map[string]float64) float64 {
	if len(conceptIDs) == 0 {
		return 0.5
	}
	sum := 0.0
	for _, conceptID := range conceptIDs {
		sum += masteryValue(conceptID, mastery)
	}
	return sum / float64(len(conceptIDs))
}

func masteryValue(conceptID string, mastery map[string]float64) float64 {
	value, ok := mastery[conceptID]
	if !ok {
		return 0.5
	}
	return value
}

func masteryTrend(avgMastery float64) string {
	if avgMastery < 0.4 {
		return "declining"
	}
	if avgMastery >= 0.7 {
		return "improving"
	}
	return "stable"
}

func countWeakConcepts(mastery map[string]float64) int {
	total := 0
	for _, value := range mastery {
		if value < 0.4 {
			total++
		}
	}
	return total
}

func averageFloatMap(values map[string]float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func percent(total int, count int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) / float64(total) * 100
}

func totalPages(total int, pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}

func compareOptionalTime(left *time.Time, right *time.Time) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return -1
	}
	if right == nil {
		return 1
	}
	if left.Before(*right) {
		return -1
	}
	if left.After(*right) {
		return 1
	}
	return 0
}

func compareOptionalTimeDesc(left *time.Time, right *time.Time) bool {
	return compareOptionalTime(left, right) > 0
}

func compareInt(left int, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareFloat(left float64, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func contentTypeValue(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "PROBLEM":
		return "problem"
	case "NOTE":
		return "note"
	case "VIDEO":
		return "video"
	case "ARTICLE":
		return "article"
	default:
		if strings.TrimSpace(value) == "" {
			return "short_answer"
		}
		return strings.ToLower(value)
	}
}

func nonEmpty(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

type listItemData struct {
	row        MistakeRow
	avgMastery float64
	errorCount int
}

type reviewCandidate struct {
	row        MistakeRow
	avgMastery float64
	errorCount int
	priority   float64
}
