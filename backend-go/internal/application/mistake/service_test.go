package mistake

import (
	"context"
	"errors"
	"testing"
	"time"

	"mathstudy/backend-go/internal/platform/maputil"
)

func TestGetMistakesSortsPaginatesAndBuildsPythonResponse(t *testing.T) {
	t1 := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t1.Add(-time.Hour)
	repo := &fakeMistakeRepo{
		rows: []MistakeRow{
			newMistakeRow("attempt-1", "content-1", []string{"algebra"}, "conceptual", t1, map[string]any{"answer": "42"}),
			newMistakeRow("attempt-2", "content-2", []string{"calculus"}, "procedural", t2, map[string]any{"answer": "x"}),
			newMistakeRow("attempt-3", "content-3", []string{"geometry"}, "logical", t3, map[string]any{"answer": "y"}),
		},
		profile: StudentProfile{MasteryVector: map[string]float64{
			"algebra":  0.3,
			"calculus": 0.85,
			"geometry": 0.5,
		}},
		hasProfile: true,
		errorCounts: map[string]int{
			"content-1": 3,
			"content-2": 1,
			"content-3": 2,
		},
	}
	service := newTestService(repo, t1)

	response, err := service.GetMistakes(context.Background(), "student-1", ListQuery{
		Page:      1,
		PageSize:  2,
		SortBy:    "error_count",
		SortOrder: "desc",
	})
	if err != nil {
		t.Fatalf("GetMistakes() error = %v", err)
	}
	if response.Pagination.Total != 3 || response.Pagination.TotalPages != 2 {
		t.Fatalf("pagination = %#v", response.Pagination)
	}
	if len(response.Items) != 2 {
		t.Fatalf("items len = %d", len(response.Items))
	}
	if response.Items[0].ID != "attempt-1" || response.Items[0].ErrorCount != 3 || response.Items[0].Attempt.CorrectAnswer != "42" {
		t.Fatalf("first item = %#v", response.Items[0])
	}
	if response.Items[1].ID != "attempt-3" || response.Items[1].Mastery.Trend != "stable" {
		t.Fatalf("second item = %#v", response.Items[1])
	}
	if response.Statistics.TotalMistakes != 3 || response.Statistics.WeakConcepts != 1 {
		t.Fatalf("statistics = %#v", response.Statistics)
	}
}

func TestGetMistakesFiltersByMasteryStatus(t *testing.T) {
	submittedAt := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeMistakeRepo{
		rows: []MistakeRow{
			newMistakeRow("weak-attempt", "weak-content", []string{"weak"}, "conceptual", submittedAt, nil),
			newMistakeRow("mastered-attempt", "mastered-content", []string{"mastered"}, "logical", submittedAt, nil),
		},
		profile: StudentProfile{MasteryVector: map[string]float64{"weak": 0.2, "mastered": 0.9}},
		errorCounts: map[string]int{
			"weak-content":     1,
			"mastered-content": 1,
		},
	}
	service := newTestService(repo, submittedAt)

	response, err := service.GetMistakes(context.Background(), "student-1", ListQuery{MasteryStatus: "weak"})
	if err != nil {
		t.Fatalf("GetMistakes() error = %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != "weak-attempt" {
		t.Fatalf("items = %#v", response.Items)
	}
}

func TestGetStatisticsBuildsDistributionAndConceptWeakness(t *testing.T) {
	now := time.Date(2026, time.April, 25, 12, 0, 0, 0, time.UTC)
	repo := &fakeMistakeRepo{
		rows: []MistakeRow{
			newMistakeRow("attempt-1", "content-1", []string{"algebra"}, "conceptual", now, nil),
			newMistakeRow("attempt-2", "content-2", []string{"algebra", "calculus"}, "conceptual", now, nil),
			newMistakeRow("attempt-3", "content-3", []string{"geometry"}, "logical", now, nil),
		},
		profile: StudentProfile{MasteryVector: map[string]float64{
			"algebra":  0.3,
			"calculus": 0.7,
			"geometry": 0.5,
		}},
		submittedAttempts: 5,
	}
	service := newTestService(repo, now)

	response, err := service.GetStatistics(context.Background(), "student-1", "week")
	if err != nil {
		t.Fatalf("GetStatistics() error = %v", err)
	}
	if response.Overview.TotalMistakes != 3 || response.Overview.TotalExercises != 5 || response.Overview.MistakeRate != 60 {
		t.Fatalf("overview = %#v", response.Overview)
	}
	if response.ErrorTypeDistribution["conceptual"].Count != 2 || response.ErrorTypeDistribution["conceptual"].Percentage != 66.7 {
		t.Fatalf("distribution = %#v", response.ErrorTypeDistribution)
	}
	if len(response.ConceptWeakness) == 0 || response.ConceptWeakness[0].ConceptID != "algebra" || response.ConceptWeakness[0].MistakeCount != 2 {
		t.Fatalf("weakness = %#v", response.ConceptWeakness)
	}
	if repo.lastFilter.DateFrom == nil || repo.lastFilter.DateTo == nil {
		t.Fatalf("time filter was not forwarded: %#v", repo.lastFilter)
	}
}

func TestGetMistakeDetailReturnsHistoryAndCachedSolution(t *testing.T) {
	submittedAt := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeMistakeRepo{
		detailRow: newMistakeRow("attempt-1", "content-1", []string{"algebra"}, "conceptual", submittedAt, map[string]any{
			"answer":         "42",
			"hints":          []any{"hint"},
			"solution_steps": []any{"step 1"},
		}),
		hasDetail: true,
		history: []MistakeHistory{
			{AttemptID: "attempt-old", SubmittedAt: stringPtr("2026-04-24T10:00:00"), IsCorrect: true, Score: 1},
		},
	}
	service := newTestService(repo, submittedAt)

	response, err := service.GetMistakeDetail(context.Background(), "student-1", "attempt-1")
	if err != nil {
		t.Fatalf("GetMistakeDetail() error = %v", err)
	}
	if response.AttemptID != "attempt-1" || response.Solution.Source != "cached" || response.Solution.Answer != "42" {
		t.Fatalf("response = %#v", response)
	}
	if len(response.Exercise.Hints) != 1 || len(response.History) != 1 {
		t.Fatalf("hints/history = %#v %#v", response.Exercise.Hints, response.History)
	}
}

func TestMarkAsMasteredRaisesRelatedConcepts(t *testing.T) {
	now := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeMistakeRepo{
		attemptContent: AttemptContent{
			Attempt: Attempt{ID: "attempt-1"},
			Content: Content{ID: "content-1", ConceptIDs: []string{"algebra", "calculus"}},
		},
		hasAttemptContent: true,
		profile:           StudentProfile{MasteryVector: map[string]float64{"algebra": 0.3, "calculus": 0.9}},
		hasProfile:        true,
	}
	service := newTestService(repo, now)

	response, err := service.MarkAsMastered(context.Background(), "student-1", "attempt-1")
	if err != nil {
		t.Fatalf("MarkAsMastered() error = %v", err)
	}
	if !response.Success || response.MasteredAt == "" {
		t.Fatalf("response = %#v", response)
	}
	if response.MasteryUpdate["algebra"] != 0.8 || response.MasteryUpdate["calculus"] != 1.0 {
		t.Fatalf("mastery update = %#v", response.MasteryUpdate)
	}
	if repo.updatedMastery["algebra"] != 0.8 || repo.updatedMastery["calculus"] != 1.0 {
		t.Fatalf("updated mastery = %#v", repo.updatedMastery)
	}
}

func TestMarkAsMasteredRequiresStudentProfile(t *testing.T) {
	repo := &fakeMistakeRepo{
		attemptContent: AttemptContent{
			Attempt: Attempt{ID: "attempt-1"},
			Content: Content{ID: "content-1", ConceptIDs: []string{"algebra"}},
		},
		hasAttemptContent: true,
		hasProfile:        false,
	}
	service := newTestService(repo, time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC))

	_, err := service.MarkAsMastered(context.Background(), "student-1", "attempt-1")
	if !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("MarkAsMastered() error = %v, want ErrProfileNotFound", err)
	}
}

func TestGetReviewExerciseSelectsHighestPriorityCandidate(t *testing.T) {
	now := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeMistakeRepo{
		rows: []MistakeRow{
			newMistakeRow("attempt-1", "content-1", []string{"algebra"}, "conceptual", now, map[string]any{"hints": []any{"hint"}}),
			newMistakeRow("attempt-2", "content-2", []string{"geometry"}, "logical", now, nil),
		},
		profile: StudentProfile{MasteryVector: map[string]float64{"algebra": 0.3, "geometry": 0.45}},
		errorCounts: map[string]int{
			"content-1": 2,
			"content-2": 4,
		},
	}
	service := newTestService(repo, now)

	response, err := service.GetReviewExercise(context.Background(), "student-1", "", "")
	if err != nil {
		t.Fatalf("GetReviewExercise() error = %v", err)
	}
	if response.Exercise.ID != "content-2" || response.Context.OriginalAttemptID != "attempt-2" {
		t.Fatalf("response = %#v", response)
	}
	if !response.Context.IsReview || response.Context.ErrorCount != 4 {
		t.Fatalf("context = %#v", response.Context)
	}
}

func TestGetReviewExerciseReturnsNotFoundWithoutCandidates(t *testing.T) {
	now := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeMistakeRepo{
		rows:        []MistakeRow{newMistakeRow("attempt-1", "content-1", []string{"algebra"}, "conceptual", now, nil)},
		profile:     StudentProfile{MasteryVector: map[string]float64{"algebra": 0.9}},
		errorCounts: map[string]int{"content-1": 1},
	}
	service := newTestService(repo, now)

	_, err := service.GetReviewExercise(context.Background(), "student-1", "", "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetReviewExercise() error = %v, want ErrNotFound", err)
	}
}

func newTestService(repo Repository, now time.Time) *Service {
	service, err := NewService(repo)
	if err != nil {
		panic(err)
	}
	service.now = func() time.Time { return now }
	return service
}

func newMistakeRow(attemptID string, contentID string, concepts []string, errorType string, submittedAt time.Time, meta map[string]any) MistakeRow {
	if meta == nil {
		meta = map[string]any{}
	}
	return MistakeRow{
		Attempt: Attempt{
			ID:               attemptID,
			ContentID:        contentID,
			StudentAnswer:    "student answer",
			StudentSteps:     []string{"step"},
			IsCorrect:        false,
			Score:            0.2,
			SubmittedAt:      &submittedAt,
			TimeSpentSeconds: 90,
		},
		Content: Content{
			ID:         contentID,
			Type:       "PROBLEM",
			Title:      "题目",
			Body:       "content",
			Difficulty: 0.5,
			ConceptIDs: concepts,
			Meta:       meta,
		},
		Diagnosis: Diagnosis{
			ErrorType:         &errorType,
			ErrorSubtype:      "sub",
			Severity:          "medium",
			Explanation:       "explanation",
			Suggestion:        "suggestion",
			RelatedConceptIDs: concepts,
			ErrorStepIndex:    intPtr(1),
		},
	}
}

type fakeMistakeRepo struct {
	rows              []MistakeRow
	lastFilter        ListFilter
	lastListQuery     ListQuery
	detailRow         MistakeRow
	hasDetail         bool
	attemptContent    AttemptContent
	hasAttemptContent bool
	history           []MistakeHistory
	profile           StudentProfile
	hasProfile        bool
	errorCounts       map[string]int
	submittedAttempts int
	updatedMastery    map[string]float64
	deleted           bool
}

func (r *fakeMistakeRepo) ListMistakes(_ context.Context, _ string, filter ListFilter) ([]MistakeRow, error) {
	r.lastFilter = filter
	return r.rows, nil
}

func (r *fakeMistakeRepo) ListMistakePage(_ context.Context, _ string, query ListQuery) ([]MistakeListRow, int, error) {
	r.lastListQuery = query
	mastery := maputil.CloneFloatMap(r.profile.MasteryVector)
	items := make([]listItemData, 0, len(r.rows))
	for _, row := range r.rows {
		avgMastery := averageMastery(row.Content.ConceptIDs, mastery)
		if !matchesMasteryStatus(avgMastery, query.MasteryStatus) {
			continue
		}
		errorCount := r.errorCounts[row.Content.ID]
		if errorCount == 0 {
			errorCount = 1
		}
		items = append(items, listItemData{row: row, avgMastery: avgMastery, errorCount: errorCount})
	}
	sortListItems(items, query.SortBy, query.SortOrder)
	total := len(items)
	start := (query.Page - 1) * query.PageSize
	end := start + query.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	rows := make([]MistakeListRow, 0, end-start)
	for _, item := range items[start:end] {
		rows = append(rows, MistakeListRow{Row: item.row, AvgMastery: item.avgMastery, ErrorCount: item.errorCount})
	}
	return rows, total, nil
}

func (r *fakeMistakeRepo) GetMistakeByAttempt(context.Context, string, string) (MistakeRow, bool, error) {
	return r.detailRow, r.hasDetail, nil
}

func (r *fakeMistakeRepo) GetAttemptContent(context.Context, string, string) (AttemptContent, bool, error) {
	return r.attemptContent, r.hasAttemptContent, nil
}

func (r *fakeMistakeRepo) ListAttemptHistory(context.Context, string, string, string) ([]MistakeHistory, error) {
	return r.history, nil
}

func (r *fakeMistakeRepo) GetProfile(context.Context, string) (StudentProfile, bool, error) {
	return r.profile, r.hasProfile, nil
}

func (r *fakeMistakeRepo) ErrorCountsByContent(context.Context, string) (map[string]int, error) {
	if r.errorCounts == nil {
		return map[string]int{}, nil
	}
	return r.errorCounts, nil
}

func (r *fakeMistakeRepo) CountSubmittedAttempts(context.Context, string, *time.Time, *time.Time) (int, error) {
	return r.submittedAttempts, nil
}

func (r *fakeMistakeRepo) UpdateProfileMastery(_ context.Context, _ string, mastery map[string]float64, _ time.Time) (bool, error) {
	r.updatedMastery = mastery
	return r.hasProfile, nil
}

func (r *fakeMistakeRepo) DeleteAttempt(context.Context, string, string) (bool, error) {
	return r.deleted, nil
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}
