package progress

import (
	"context"
	"testing"
	"time"
)

func TestGetOverviewCombinesProfileDKTAndAttemptStats(t *testing.T) {
	now := time.Date(2026, time.April, 8, 15, 0, 0, 0, time.UTC)
	latest := now.Add(-2 * time.Hour)
	repo := &fakeProgressRepo{
		profile: StudentProfile{
			TotalExercises: 10,
			CorrectCount:   7,
			MasteryVector:  map[string]float64{"fallback": 0.82},
		},
		hasProfile:             true,
		totalStudySeconds:      7200,
		todayStudySeconds:      1800,
		todayAttempts:          3,
		latestAttemptStartedAt: &latest,
		submittedDays: []time.Time{
			now,
			now.AddDate(0, 0, -1),
			now.AddDate(0, 0, -2),
		},
		masteryStates: []MasteryState{
			{ConceptID: "limit", Mastery: 0.91, Confidence: 0.7, AttemptCount: 4},
			{ConceptID: "weak", Mastery: 0.4, Confidence: 0.2, AttemptCount: 1},
		},
	}
	service := newTestService(repo, now)

	overview, err := service.GetOverview(context.Background(), "student-1")
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview.TotalExercises != 10 || overview.CorrectCount != 7 || overview.CorrectRate != 70 {
		t.Fatalf("overview counters = %#v", overview)
	}
	if overview.StudyMinutes != 120 || overview.TodayStats.StudyMinutes != 30 || overview.TodayStats.ExercisesCompleted != 3 {
		t.Fatalf("overview time stats = %#v", overview)
	}
	if overview.StreakDays != 3 || overview.MasteredConcepts != 1 {
		t.Fatalf("overview mastery stats = %#v", overview)
	}
	if overview.RecentContent == nil || overview.RecentContent.LastAccessed == "" {
		t.Fatalf("recent content = %#v", overview.RecentContent)
	}
}

func TestGetStatisticsBuildsWeekRangeAndErrorDistribution(t *testing.T) {
	now := time.Date(2026, time.April, 8, 15, 0, 0, 0, time.UTC)
	repo := &fakeProgressRepo{
		dayStats: []PeriodStat{
			{
				Date:             time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC),
				Exercises:        4,
				CorrectExercises: 3,
				StudySeconds:     1800,
			},
		},
		errorCounts: map[string]int{"conceptual": 2, "procedural": 1},
	}
	service := newTestService(repo, now)

	stats, err := service.GetStatistics(context.Background(), "student-1", "week")
	if err != nil {
		t.Fatalf("GetStatistics() error = %v", err)
	}
	if stats.Interval != "day" || stats.RangeDays != 3 || stats.StartDate != "2026-04-06" || stats.EndDate != "2026-04-08" {
		t.Fatalf("range = %#v", stats)
	}
	if len(stats.Daily) != 3 {
		t.Fatalf("daily len = %d", len(stats.Daily))
	}
	if stats.Daily[0].Date != "2026-04-06" || stats.Daily[0].Exercises != 4 || stats.Daily[0].StudyMinutes != 30 {
		t.Fatalf("first daily stat = %#v", stats.Daily[0])
	}
	if stats.ErrorTypeDistribution["conceptual"].Percentage != 66.7 || stats.ErrorTypeDistribution["procedural"].Percentage != 33.3 {
		t.Fatalf("error distribution = %#v", stats.ErrorTypeDistribution)
	}
}

func TestGetLearningPathUsesGraphOrderAndNodeStatus(t *testing.T) {
	now := time.Date(2026, time.April, 8, 15, 0, 0, 0, time.UTC)
	chapter := "第一章"
	repo := &fakeProgressRepo{
		hasProfile: true,
		masteryStates: []MasteryState{
			{ConceptID: "a", Mastery: 0.9, Confidence: 0.7, AttemptCount: 3},
			{ConceptID: "b", Mastery: 0.6, Confidence: 0.4, AttemptCount: 0},
		},
		nodes: []KnowledgeNode{
			{ID: "a", Name: "导数", NodeType: "CONCEPT", Description: "desc-a", Chapter: &chapter, Difficulty: 0.5},
			{ID: "b", Name: "极限", NodeType: "CONCEPT", Description: "desc-b", Chapter: &chapter, Difficulty: 0.4},
		},
		relations: []KnowledgeRelation{
			{ID: "r1", SourceID: "a", TargetID: "b", RelationType: "HAS_PREREQUISITE"},
		},
	}
	service := newTestService(repo, now)

	path, err := service.GetLearningPath(context.Background(), "student-1", "")
	if err != nil {
		t.Fatalf("GetLearningPath() error = %v", err)
	}
	if len(path.Path) != 2 {
		t.Fatalf("path len = %d", len(path.Path))
	}
	if path.Path[0].ID != "b" || path.Path[0].Status != "available" {
		t.Fatalf("first path item = %#v", path.Path[0])
	}
	if path.Path[1].ID != "a" || path.Path[1].Status != "completed" {
		t.Fatalf("second path item = %#v", path.Path[1])
	}
	if path.EstimatedExercises != 5 || path.Statistics.Completed != 1 || path.Statistics.Progress != 0.5 {
		t.Fatalf("path summary = %#v", path)
	}
}

func TestGetLearningPathPrunesTargetAndLocksMissingPrerequisite(t *testing.T) {
	now := time.Date(2026, time.April, 8, 15, 0, 0, 0, time.UTC)
	chapter := "多元微积分"
	repo := &fakeProgressRepo{
		hasProfile: true,
		masteryStates: []MasteryState{
			{ConceptID: "derivative", Mastery: 0.3, Confidence: 0.2, AttemptCount: 0},
			{ConceptID: "partial", Mastery: 0.2, Confidence: 0.1, AttemptCount: 0},
			{ConceptID: "integral", Mastery: 0.95, Confidence: 0.8, AttemptCount: 5},
		},
		nodes: []KnowledgeNode{
			{ID: "derivative", Name: "导数定义", NodeType: "CONCEPT", Description: "先修", Chapter: &chapter, Difficulty: 0.3},
			{ID: "partial", Name: "偏导数", NodeType: "CONCEPT", Description: "目标", Chapter: &chapter, Difficulty: 0.6},
			{ID: "integral", Name: "定积分", NodeType: "CONCEPT", Description: "无关", Difficulty: 0.5},
		},
		relations: []KnowledgeRelation{
			{ID: "r1", SourceID: "partial", TargetID: "derivative", RelationType: "HAS_PREREQUISITE"},
		},
	}
	service := newTestService(repo, now)

	path, err := service.GetLearningPath(context.Background(), "student-1", "partial")
	if err != nil {
		t.Fatalf("GetLearningPath() error = %v", err)
	}
	if len(path.Path) != 2 || path.Path[0].ID != "derivative" || path.Path[1].ID != "partial" {
		t.Fatalf("path = %#v", path.Path)
	}
	if path.Path[1].Status != "locked" || len(path.Path[1].LockedBy) != 1 || path.Path[1].LockedBy[0] != "derivative" {
		t.Fatalf("locked target = %#v", path.Path[1])
	}
}

func TestGetClassRankingRanksByStudyTimeThenAttemptCount(t *testing.T) {
	repo := &fakeProgressRepo{
		classStudentIDs: []string{"student-1", "student-2", "student-3"},
		classTeacherID:  "teacher-1",
		inClass:         true,
		studentAttemptStats: map[string]StudentAttemptStats{
			"student-1": {StudySeconds: 3600, AttemptCount: 3},
			"student-2": {StudySeconds: 3600, AttemptCount: 5},
			"student-3": {StudySeconds: 600, AttemptCount: 10},
		},
	}
	service := newTestService(repo, time.Date(2026, time.April, 8, 15, 0, 0, 0, time.UTC))

	ranking, err := service.GetClassRanking(context.Background(), "student-1")
	if err != nil {
		t.Fatalf("GetClassRanking() error = %v", err)
	}
	if !ranking.InClass || ranking.Rank == nil || *ranking.Rank != 2 || ranking.Total != 3 {
		t.Fatalf("ranking = %#v", ranking)
	}
	if ranking.Percentile == nil || *ranking.Percentile != 66.7 {
		t.Fatalf("percentile = %#v", ranking.Percentile)
	}
	if repo.attemptStatsTeacherID != "teacher-1" {
		t.Fatalf("attempt stats teacher ID = %q, want teacher-1", repo.attemptStatsTeacherID)
	}
	if len(repo.attemptStatsStudentIDs) != 3 || repo.attemptStatsStudentIDs[0] != "student-1" {
		t.Fatalf("attempt stats student IDs = %#v", repo.attemptStatsStudentIDs)
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

type fakeProgressRepo struct {
	profile                StudentProfile
	hasProfile             bool
	attemptTotal           int
	attemptCorrect         int
	totalStudySeconds      int
	todayStudySeconds      int
	todayAttempts          int
	latestAttemptStartedAt *time.Time
	submittedDays          []time.Time
	masteryStates          []MasteryState
	nodes                  []KnowledgeNode
	relations              []KnowledgeRelation
	dayStats               []PeriodStat
	weekStats              []PeriodStat
	errorCounts            map[string]int
	classStudentIDs        []string
	classTeacherID         string
	inClass                bool
	studentAttemptStats    map[string]StudentAttemptStats
	attemptStatsTeacherID  string
	attemptStatsStudentIDs []string
	chapters               []string
}

func (r *fakeProgressRepo) GetProfile(context.Context, string) (StudentProfile, bool, error) {
	return r.profile, r.hasProfile, nil
}

func (r *fakeProgressRepo) GetAttemptTotals(context.Context, string) (int, int, error) {
	return r.attemptTotal, r.attemptCorrect, nil
}

func (r *fakeProgressRepo) SumStudySeconds(_ context.Context, _ string, since *time.Time) (int, error) {
	if since != nil {
		return r.todayStudySeconds, nil
	}
	return r.totalStudySeconds, nil
}

func (r *fakeProgressRepo) CountAttemptsStartedSince(context.Context, string, time.Time) (int, error) {
	return r.todayAttempts, nil
}

func (r *fakeProgressRepo) LatestAttemptStartedAt(context.Context, string) (*time.Time, error) {
	return r.latestAttemptStartedAt, nil
}

func (r *fakeProgressRepo) ListSubmittedAttemptDays(context.Context, string, int) ([]time.Time, error) {
	return r.submittedDays, nil
}

func (r *fakeProgressRepo) ListMasteryStates(context.Context, string, []string) ([]MasteryState, error) {
	return r.masteryStates, nil
}

func (r *fakeProgressRepo) ListKnowledgeNodes(_ context.Context, filter KnowledgeNodeFilter) ([]KnowledgeNode, error) {
	if filter.NodeType == "" && filter.Chapter == "" && filter.Search == "" {
		return r.nodes, nil
	}
	filtered := []KnowledgeNode{}
	for _, node := range r.nodes {
		if filter.NodeType != "" && graphNodeType(node.NodeType) != filter.NodeType {
			continue
		}
		if filter.Chapter != "" && (node.Chapter == nil || *node.Chapter != filter.Chapter) {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered, nil
}

func (r *fakeProgressRepo) ListKnowledgeRelations(context.Context) ([]KnowledgeRelation, error) {
	return r.relations, nil
}

func (r *fakeProgressRepo) ListLearningStatsByDay(context.Context, string, time.Time, time.Time) ([]PeriodStat, error) {
	return r.dayStats, nil
}

func (r *fakeProgressRepo) ListLearningStatsByWeek(context.Context, string, time.Time, time.Time) ([]PeriodStat, error) {
	return r.weekStats, nil
}

func (r *fakeProgressRepo) CountErrorsByType(context.Context, string, time.Time, time.Time) (map[string]int, error) {
	return r.errorCounts, nil
}

func (r *fakeProgressRepo) ListClassStudentIDs(context.Context, string) ([]string, string, bool, error) {
	return r.classStudentIDs, r.classTeacherID, r.inClass, nil
}

func (r *fakeProgressRepo) AttemptStatsForStudents(_ context.Context, teacherID string, studentIDs []string) (map[string]StudentAttemptStats, error) {
	r.attemptStatsTeacherID = teacherID
	r.attemptStatsStudentIDs = append([]string(nil), studentIDs...)
	return r.studentAttemptStats, nil
}

func (r *fakeProgressRepo) DistinctChapters(context.Context) ([]string, error) {
	return r.chapters, nil
}
