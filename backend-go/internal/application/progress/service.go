package progress

import (
	"container/heap"
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"mathstudy/backend-go/internal/platform/maputil"
)

const (
	// StreakLookbackDays matches the Python progress service lookback window.
	StreakLookbackDays = 365

	masteryThreshold  = 0.85
	dktRetentionFloor = 0.05
	dktModelName      = "dkt-sakt-lite"
)

// Repository is the persistence surface required by progress use cases.
type Repository interface {
	GetProfile(context.Context, string) (StudentProfile, bool, error)
	GetAttemptTotals(context.Context, string) (int, int, error)
	SumStudySeconds(context.Context, string, *time.Time) (int, error)
	CountAttemptsStartedSince(context.Context, string, time.Time) (int, error)
	LatestAttemptStartedAt(context.Context, string) (*time.Time, error)
	ListSubmittedAttemptDays(context.Context, string, int) ([]time.Time, error)
	ListMasteryStates(context.Context, string, []string) ([]MasteryState, error)
	ListKnowledgeNodes(context.Context, KnowledgeNodeFilter) ([]KnowledgeNode, error)
	ListKnowledgeRelations(context.Context) ([]KnowledgeRelation, error)
	ListLearningStatsByDay(context.Context, string, time.Time, time.Time) ([]PeriodStat, error)
	ListLearningStatsByWeek(context.Context, string, time.Time, time.Time) ([]PeriodStat, error)
	CountErrorsByType(context.Context, string, time.Time, time.Time) (map[string]int, error)
	ListClassStudentIDs(context.Context, string) ([]string, bool, error)
	AttemptStatsForStudents(context.Context, []string) (map[string]StudentAttemptStats, error)
	DistinctChapters(context.Context) ([]string, error)
}

// StudentProfile mirrors the progress fields in student_profiles.
type StudentProfile struct {
	TotalExercises int
	CorrectCount   int
	MasteryVector  map[string]float64
}

// MasteryState stores DKT state used by read-side progress queries.
type MasteryState struct {
	ConceptID     string
	Mastery       float64
	Confidence    float64
	AttemptCount  int
	LastAttemptAt *time.Time
}

// KnowledgeNode is a read model for knowledge graph nodes.
type KnowledgeNode struct {
	ID          string
	Name        string
	NodeType    string
	Description string
	Chapter     *string
	Difficulty  float64
	CreatedAt   time.Time
}

// KnowledgeNodeFilter stores optional filters for graph queries.
type KnowledgeNodeFilter struct {
	Chapter  string
	NodeType string
	Search   string
}

// KnowledgeRelation is a read model for knowledge graph relations.
type KnowledgeRelation struct {
	ID           string
	SourceID     string
	TargetID     string
	RelationType string
	CreatedAt    time.Time
}

// PeriodStat stores one daily or weekly aggregate.
type PeriodStat struct {
	Date             time.Time
	Exercises        int
	CorrectExercises int
	StudySeconds     int
}

// StudentAttemptStats stores ranking inputs for one student.
type StudentAttemptStats struct {
	StudySeconds int
	AttemptCount int
}

// Overview is the /progress/overview response.
type Overview struct {
	TotalExercises   int            `json:"total_exercises"`
	CorrectCount     int            `json:"correct_count"`
	CorrectRate      float64        `json:"correct_rate"`
	StudyMinutes     int            `json:"study_time_minutes"`
	StreakDays       int            `json:"streak_days"`
	MasteredConcepts int            `json:"mastered_concepts"`
	TodayStats       TodayStats     `json:"today_stats"`
	RecentContent    *RecentContent `json:"recent_content"`
}

// TodayStats stores today's progress counters.
type TodayStats struct {
	StudyMinutes       int `json:"study_time_minutes"`
	ExercisesCompleted int `json:"exercises_completed"`
}

// RecentContent stores the latest attempt timestamp.
type RecentContent struct {
	LastAccessed string `json:"last_accessed"`
}

// MasteryResponse is the /progress/mastery response.
type MasteryResponse struct {
	Topics []MasteryTopic `json:"topics"`
	Model  string         `json:"model"`
}

// MasteryTopic stores one concept mastery row.
type MasteryTopic struct {
	Topic      string  `json:"topic"`
	Mastery    float64 `json:"mastery"`
	Exercises  int     `json:"exercises"`
	Confidence float64 `json:"confidence"`
}

// PathResponse is the /progress/path response.
type PathResponse struct {
	Path               []PathItem     `json:"path"`
	EstimatedExercises int            `json:"estimated_exercises"`
	Statistics         PathStatistics `json:"statistics"`
}

// PathItem stores one personalized learning path node.
type PathItem struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Chapter        *string  `json:"chapter"`
	Status         string   `json:"status"`
	LockedBy       []string `json:"locked_by,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	Mastery        float64  `json:"mastery"`
	Confidence     float64  `json:"confidence"`
	Exercises      int      `json:"exercises"`
	Difficulty     float64  `json:"difficulty"`
}

// PathStatistics stores learning path aggregate counters.
type PathStatistics struct {
	Total     int     `json:"total"`
	Completed int     `json:"completed"`
	Progress  float64 `json:"progress"`
}

// GraphResponse is the /progress/knowledge-graph response.
type GraphResponse struct {
	Nodes      []GraphNode     `json:"nodes"`
	Edges      []GraphEdge     `json:"edges"`
	Statistics GraphStatistics `json:"statistics"`
}

// GraphNode stores one frontend graph node.
type GraphNode struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Type        string  `json:"type"`
	Mastery     float64 `json:"mastery"`
	Chapter     *string `json:"chapter"`
	Description string  `json:"description"`
}

// GraphEdge stores one frontend graph edge.
type GraphEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
}

// GraphStatistics stores graph summary counters.
type GraphStatistics struct {
	TotalNodes     int     `json:"total_nodes"`
	MasteredNodes  int     `json:"mastered_nodes"`
	OverallMastery float64 `json:"overall_mastery"`
}

// StatisticsResponse is the /progress/statistics response.
type StatisticsResponse struct {
	RangeDays             int                              `json:"range_days"`
	Interval              string                           `json:"interval"`
	StartDate             string                           `json:"start_date"`
	EndDate               string                           `json:"end_date"`
	Daily                 []DailyStat                      `json:"daily"`
	ErrorTypeDistribution map[string]ErrorTypeDistribution `json:"error_type_distribution"`
}

// DailyStat stores one daily or weekly chart point.
type DailyStat struct {
	Date             string `json:"date"`
	Exercises        int    `json:"exercises"`
	CorrectExercises int    `json:"correct_exercises"`
	StudyMinutes     int    `json:"study_minutes"`
}

// ErrorTypeDistribution stores error count and percentage.
type ErrorTypeDistribution struct {
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ClassRankingResponse is the /progress/class-ranking response.
type ClassRankingResponse struct {
	InClass    bool     `json:"in_class"`
	Rank       *int     `json:"rank"`
	Total      int      `json:"total"`
	Percentile *float64 `json:"percentile"`
}

// Service implements student progress read use cases.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService creates a progress service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("progress repository is nil")
	}
	return &Service{
		repo: repo,
		now:  time.Now,
	}, nil
}

// GetOverview returns the student learning progress overview.
func (s *Service) GetOverview(ctx context.Context, userID string) (Overview, error) {
	profile, hasProfile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return Overview{}, err
	}

	totalExercises := profile.TotalExercises
	correctCount := profile.CorrectCount
	if !hasProfile {
		totalExercises, correctCount, err = s.repo.GetAttemptTotals(ctx, userID)
		if err != nil {
			return Overview{}, err
		}
	}

	mastery, _, _, err := s.masteryDetails(ctx, userID, profile.MasteryVector, nil)
	if err != nil {
		return Overview{}, err
	}
	masteredConcepts := 0
	for _, value := range mastery {
		if value >= masteryThreshold {
			masteredConcepts++
		}
	}

	totalSeconds, err := s.repo.SumStudySeconds(ctx, userID, nil)
	if err != nil {
		return Overview{}, err
	}
	now := s.now()
	todayStart := startOfDay(now)
	todaySeconds, err := s.repo.SumStudySeconds(ctx, userID, &todayStart)
	if err != nil {
		return Overview{}, err
	}
	todayAttempts, err := s.repo.CountAttemptsStartedSince(ctx, userID, todayStart)
	if err != nil {
		return Overview{}, err
	}
	streakDays, err := s.calculateStreakDays(ctx, userID)
	if err != nil {
		return Overview{}, err
	}
	latest, err := s.repo.LatestAttemptStartedAt(ctx, userID)
	if err != nil {
		return Overview{}, err
	}

	var recent *RecentContent
	if latest != nil {
		recent = &RecentContent{LastAccessed: latest.Format("2006-01-02T15:04:05.999999")}
	}

	return Overview{
		TotalExercises:   totalExercises,
		CorrectCount:     correctCount,
		CorrectRate:      round1(percent(totalExercises, correctCount)),
		StudyMinutes:     totalSeconds / 60,
		StreakDays:       streakDays,
		MasteredConcepts: masteredConcepts,
		TodayStats: TodayStats{
			StudyMinutes:       todaySeconds / 60,
			ExercisesCompleted: todayAttempts,
		},
		RecentContent: recent,
	}, nil
}

// GetMasteryVector returns the concept mastery vector.
func (s *Service) GetMasteryVector(ctx context.Context, userID string) (MasteryResponse, error) {
	profile, _, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return MasteryResponse{}, err
	}
	mastery, confidence, attempts, err := s.masteryDetails(ctx, userID, profile.MasteryVector, nil)
	if err != nil {
		return MasteryResponse{}, err
	}

	keys := maputil.SortedFloatKeys(mastery)
	topics := make([]MasteryTopic, 0, len(keys))
	for _, key := range keys {
		topics = append(topics, MasteryTopic{
			Topic:      key,
			Mastery:    mastery[key],
			Exercises:  attempts[key],
			Confidence: confidence[key],
		})
	}
	return MasteryResponse{Topics: topics, Model: dktModelName}, nil
}

// GetLearningPath returns a personalized learning path.
func (s *Service) GetLearningPath(ctx context.Context, userID string, target string) (PathResponse, error) {
	profile, _, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return PathResponse{}, err
	}
	mastery, confidence, attempts, err := s.masteryDetails(ctx, userID, profile.MasteryVector, nil)
	if err != nil {
		return PathResponse{}, err
	}

	nodes, err := s.repo.ListKnowledgeNodes(ctx, KnowledgeNodeFilter{})
	if err != nil {
		return PathResponse{}, err
	}
	relations, err := s.repo.ListKnowledgeRelations(ctx)
	if err != nil {
		return PathResponse{}, err
	}

	nodeMap := make(map[string]KnowledgeNode)
	for _, node := range nodes {
		if graphNodeType(node.NodeType) == "" {
			continue
		}
		nodeMap[node.ID] = node
	}
	if len(nodeMap) == 0 {
		return PathResponse{Path: []PathItem{}, Statistics: PathStatistics{}}, nil
	}

	prereqEdges := make(map[string]map[string]struct{}, len(nodeMap))
	dependents := make(map[string]map[string]struct{}, len(nodeMap))
	for id := range nodeMap {
		prereqEdges[id] = map[string]struct{}{}
		dependents[id] = map[string]struct{}{}
	}
	for _, rel := range relations {
		if rel.RelationType != "HAS_PREREQUISITE" {
			continue
		}
		if _, ok := nodeMap[rel.SourceID]; !ok {
			continue
		}
		if _, ok := nodeMap[rel.TargetID]; !ok {
			continue
		}
		prereqEdges[rel.SourceID][rel.TargetID] = struct{}{}
		dependents[rel.TargetID][rel.SourceID] = struct{}{}
	}

	sortedIDs := topologicalLearningOrder(nodeMap, prereqEdges, dependents, mastery)
	if target != "" {
		targetNodes := selectTargetNodes(target, nodeMap)
		if len(targetNodes) > 0 {
			needed := collectLearningSubgraph(targetNodes, prereqEdges, mastery)
			filtered := make([]string, 0, len(sortedIDs))
			for _, id := range sortedIDs {
				if _, ok := needed[id]; ok {
					filtered = append(filtered, id)
				}
			}
			sortedIDs = filtered
		}
	}

	path := make([]PathItem, 0, len(sortedIDs))
	completed := 0
	estimatedRemaining := 0
	for _, id := range sortedIDs {
		node := nodeMap[id]
		masteryValue := mastery[id]
		confidenceValue := confidence[id]
		exerciseCount := attempts[id]
		lockedBy := lockedPrerequisites(id, prereqEdges, mastery, confidence)
		status := learningNodeStatus(id, lockedBy, mastery, confidenceValue, exerciseCount)
		if status == "completed" {
			completed++
		} else if remaining := 5 - exerciseCount; remaining > 0 {
			estimatedRemaining += remaining
		}
		path = append(path, PathItem{
			ID:             id,
			Title:          node.Name,
			Description:    node.Description,
			Chapter:        node.Chapter,
			Status:         status,
			LockedBy:       lockedBy,
			Recommendation: learningRecommendation(status),
			Mastery:        round4(masteryValue),
			Confidence:     round4(confidenceValue),
			Exercises:      exerciseCount,
			Difficulty:     node.Difficulty,
		})
	}

	return PathResponse{
		Path:               path,
		EstimatedExercises: estimatedRemaining,
		Statistics: PathStatistics{
			Total:     len(path),
			Completed: completed,
			Progress:  round2(float64(completed) / float64(maxInt(len(path), 1))),
		},
	}, nil
}

// GetKnowledgeGraphView returns frontend graph data.
func (s *Service) GetKnowledgeGraphView(ctx context.Context, userID string, filter KnowledgeNodeFilter) (GraphResponse, error) {
	profile, _, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return GraphResponse{}, err
	}
	mastery, _, _, err := s.masteryDetails(ctx, userID, profile.MasteryVector, nil)
	if err != nil {
		return GraphResponse{}, err
	}

	nodes, err := s.repo.ListKnowledgeNodes(ctx, filter)
	if err != nil {
		return GraphResponse{}, err
	}
	relations, err := s.repo.ListKnowledgeRelations(ctx)
	if err != nil {
		return GraphResponse{}, err
	}

	nodeIDs := make(map[string]struct{}, len(nodes))
	graphNodes := make([]GraphNode, 0, len(nodes))
	for _, node := range nodes {
		nodeType := graphNodeType(node.NodeType)
		if nodeType == "" {
			continue
		}
		nodeIDs[node.ID] = struct{}{}
		graphNodes = append(graphNodes, GraphNode{
			ID:          node.ID,
			Label:       node.Name,
			Type:        nodeType,
			Mastery:     mastery[node.ID],
			Chapter:     node.Chapter,
			Description: node.Description,
		})
	}

	graphEdges := make([]GraphEdge, 0, len(relations))
	for _, rel := range relations {
		if _, ok := nodeIDs[rel.SourceID]; !ok {
			continue
		}
		if _, ok := nodeIDs[rel.TargetID]; !ok {
			continue
		}
		relation := graphRelationType(rel.RelationType)
		if relation == "" {
			continue
		}
		graphEdges = append(graphEdges, GraphEdge{
			Source:   rel.SourceID,
			Target:   rel.TargetID,
			Relation: relation,
		})
	}

	mastered := 0
	sum := 0.0
	for _, node := range graphNodes {
		sum += node.Mastery
		if node.Mastery >= masteryThreshold {
			mastered++
		}
	}
	overall := 0.0
	if len(graphNodes) > 0 {
		overall = round2(sum / float64(len(graphNodes)))
	}
	return GraphResponse{
		Nodes: graphNodes,
		Edges: graphEdges,
		Statistics: GraphStatistics{
			TotalNodes:     len(graphNodes),
			MasteredNodes:  mastered,
			OverallMastery: overall,
		},
	}, nil
}

// GetStatistics returns learning activity statistics.
func (s *Service) GetStatistics(ctx context.Context, userID string, rangeType string) (StatisticsResponse, error) {
	now := s.now()
	today := startOfDay(now)
	startDate := today.AddDate(0, 0, -weekdayMondayIndex(today))
	rangeDays := 7
	interval := "day"

	switch rangeType {
	case "month":
		startDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		rangeDays = int(today.Sub(startDate).Hours()/24) + 1
	case "semester":
		interval = "week"
		if today.Month() >= time.September {
			startDate = time.Date(today.Year(), time.September, 1, 0, 0, 0, 0, today.Location())
		} else if today.Month() == time.January {
			startDate = time.Date(today.Year()-1, time.September, 1, 0, 0, 0, 0, today.Location())
		} else {
			startDate = time.Date(today.Year(), time.February, 1, 0, 0, 0, 0, today.Location())
		}
		rangeDays = int(today.Sub(startDate).Hours()/24) + 1
	case "all":
		interval = "week"
		rangeDays = 365
		startDate = today.AddDate(0, 0, -364)
	default:
		rangeDays = minInt(7, int(today.Sub(startDate).Hours()/24)+1)
	}

	endDate := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), today.Location())
	var rows []PeriodStat
	var err error
	if interval == "day" {
		rows, err = s.repo.ListLearningStatsByDay(ctx, userID, startDate, endDate)
	} else {
		rows, err = s.repo.ListLearningStatsByWeek(ctx, userID, startDate, endDate)
	}
	if err != nil {
		return StatisticsResponse{}, err
	}

	daily := buildDailyStats(interval, startDate, today, rangeDays, rows)
	errorCounts, err := s.repo.CountErrorsByType(ctx, userID, startDate, endDate)
	if err != nil {
		return StatisticsResponse{}, err
	}
	distribution := buildErrorDistribution(errorCounts)

	return StatisticsResponse{
		RangeDays:             rangeDays,
		Interval:              interval,
		StartDate:             dateString(startDate),
		EndDate:               dateString(today),
		Daily:                 daily,
		ErrorTypeDistribution: distribution,
	}, nil
}

// GetClassRanking returns the current student's class ranking.
func (s *Service) GetClassRanking(ctx context.Context, userID string) (ClassRankingResponse, error) {
	studentIDs, inClass, err := s.repo.ListClassStudentIDs(ctx, userID)
	if err != nil {
		return ClassRankingResponse{}, err
	}
	if !inClass {
		return ClassRankingResponse{InClass: false, Total: 0}, nil
	}
	if len(studentIDs) == 0 {
		return ClassRankingResponse{InClass: true, Total: 0}, nil
	}

	stats, err := s.repo.AttemptStatsForStudents(ctx, studentIDs)
	if err != nil {
		return ClassRankingResponse{}, err
	}
	rankingRows := make([]rankingRow, 0, len(studentIDs))
	for _, id := range studentIDs {
		stat := stats[id]
		rankingRows = append(rankingRows, rankingRow{
			StudentID:    id,
			StudySeconds: stat.StudySeconds,
			AttemptCount: stat.AttemptCount,
		})
	}
	sort.Slice(rankingRows, func(i, j int) bool {
		if rankingRows[i].StudySeconds == rankingRows[j].StudySeconds {
			return rankingRows[i].AttemptCount > rankingRows[j].AttemptCount
		}
		return rankingRows[i].StudySeconds > rankingRows[j].StudySeconds
	})

	rank := 1
	for i, row := range rankingRows {
		if row.StudentID == userID {
			rank = i + 1
			break
		}
	}
	total := len(rankingRows)
	percentile := round1((1.0 - float64(rank-1)/float64(total)) * 100.0)
	return ClassRankingResponse{
		InClass:    true,
		Rank:       &rank,
		Total:      total,
		Percentile: &percentile,
	}, nil
}

// GetChapters returns distinct knowledge graph chapters.
func (s *Service) GetChapters(ctx context.Context) ([]string, error) {
	return s.repo.DistinctChapters(ctx)
}

func (s *Service) masteryDetails(ctx context.Context, userID string, fallback map[string]float64, conceptIDs []string) (map[string]float64, map[string]float64, map[string]int, error) {
	mastery := maputil.CloneFloatMap(fallback)
	confidence := map[string]float64{}
	attempts := map[string]int{}

	states, err := s.repo.ListMasteryStates(ctx, userID, conceptIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	now := s.now()
	for _, state := range states {
		rawMastery := state.Mastery
		if state.LastAttemptAt != nil {
			daysSince := now.Sub(*state.LastAttemptAt).Hours() / 24.0
			rawMastery = applyForgetting(rawMastery, daysSince, dktRetentionFloor)
		}
		mastery[state.ConceptID] = round4(rawMastery)
		confidence[state.ConceptID] = round4(state.Confidence)
		attempts[state.ConceptID] = state.AttemptCount
	}

	for _, conceptID := range conceptIDs {
		if _, ok := mastery[conceptID]; !ok {
			mastery[conceptID] = 0.5
		}
	}
	return mastery, confidence, attempts, nil
}

func (s *Service) calculateStreakDays(ctx context.Context, userID string) (int, error) {
	days, err := s.repo.ListSubmittedAttemptDays(ctx, userID, StreakLookbackDays)
	if err != nil {
		return 0, err
	}
	if len(days) == 0 {
		return 0, nil
	}
	activeDays := make(map[string]struct{}, len(days))
	for _, day := range days {
		activeDays[dateString(day)] = struct{}{}
	}
	streak := 0
	current := startOfDay(s.now())
	for {
		if _, ok := activeDays[dateString(current)]; !ok {
			return streak, nil
		}
		streak++
		current = current.AddDate(0, 0, -1)
	}
}

func topologicalLearningOrder(nodes map[string]KnowledgeNode, prereqEdges, dependents map[string]map[string]struct{}, mastery map[string]float64) []string {
	inDegree := make(map[string]int, len(nodes))
	pq := &learningHeap{}
	for id := range nodes {
		inDegree[id] = len(prereqEdges[id])
		if inDegree[id] == 0 {
			heap.Push(pq, learningHeapItem{id: id, mastery: mastery[id]})
		}
	}

	heap.Init(pq)
	sortedIDs := make([]string, 0, len(nodes))
	seen := make(map[string]struct{}, len(nodes))
	for pq.Len() > 0 {
		item := heap.Pop(pq).(learningHeapItem)
		sortedIDs = append(sortedIDs, item.id)
		seen[item.id] = struct{}{}
		for dependent := range dependents[item.id] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				heap.Push(pq, learningHeapItem{id: dependent, mastery: mastery[dependent]})
			}
		}
	}

	remaining := make([]string, 0, len(nodes)-len(sortedIDs))
	for id := range nodes {
		if _, ok := seen[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	sort.Slice(remaining, func(i, j int) bool {
		left := mastery[remaining[i]]
		right := mastery[remaining[j]]
		if left == right {
			return remaining[i] < remaining[j]
		}
		return left < right
	})
	return append(sortedIDs, remaining...)
}

func collectPrerequisites(target string, prereqEdges map[string]map[string]struct{}) map[string]struct{} {
	needed := map[string]struct{}{}
	stack := []string{target}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := needed[current]; ok {
			continue
		}
		needed[current] = struct{}{}
		for prereq := range prereqEdges[current] {
			stack = append(stack, prereq)
		}
	}
	return needed
}

func selectTargetNodes(target string, nodes map[string]KnowledgeNode) []string {
	normalized := normalizeTarget(target)
	if normalized == "" {
		return nil
	}
	if _, ok := nodes[target]; ok {
		return []string{target}
	}
	selected := []string{}
	for id, node := range nodes {
		if strings.Contains(normalizeTarget(node.Name), normalized) ||
			strings.Contains(normalizeTarget(node.Description), normalized) ||
			(node.Chapter != nil && strings.Contains(normalizeTarget(*node.Chapter), normalized)) {
			selected = append(selected, id)
		}
	}
	sort.Strings(selected)
	return selected
}

func collectLearningSubgraph(targets []string, prereqEdges map[string]map[string]struct{}, mastery map[string]float64) map[string]struct{} {
	needed := map[string]struct{}{}
	for _, target := range targets {
		if mastery[target] < masteryThreshold {
			for id := range collectPrerequisites(target, prereqEdges) {
				needed[id] = struct{}{}
			}
			continue
		}
		for prereq := range prereqEdges[target] {
			if mastery[prereq] < masteryThreshold {
				for id := range collectPrerequisites(prereq, prereqEdges) {
					needed[id] = struct{}{}
				}
			}
		}
	}
	if len(needed) == 0 {
		for _, target := range targets {
			needed[target] = struct{}{}
		}
	}
	return needed
}

func lockedPrerequisites(id string, prereqEdges map[string]map[string]struct{}, mastery map[string]float64, confidence map[string]float64) []string {
	locked := []string{}
	for prereq := range prereqEdges[id] {
		if mastery[prereq] < masteryThreshold || confidence[prereq] < 0.5 {
			locked = append(locked, prereq)
		}
	}
	sort.Strings(locked)
	return locked
}

func learningNodeStatus(id string, lockedBy []string, mastery map[string]float64, confidence float64, exercises int) string {
	if mastery[id] >= masteryThreshold && confidence >= 0.5 {
		return "completed"
	}
	if len(lockedBy) > 0 {
		return "locked"
	}
	if exercises > 0 {
		return "current"
	}
	return "available"
}

func learningRecommendation(status string) string {
	switch status {
	case "locked":
		return "先完成先修节点的微课视频和基础练习"
	case "current":
		return "继续进行当前知识点的针对性练习"
	case "available":
		return "可以开始该知识点的入门练习"
	default:
		return ""
	}
}

func buildDailyStats(interval string, startDate time.Time, today time.Time, rangeDays int, rows []PeriodStat) []DailyStat {
	statsByDate := make(map[string]PeriodStat, len(rows))
	for _, row := range rows {
		statsByDate[dateString(row.Date)] = row
	}

	if interval == "day" {
		daily := make([]DailyStat, 0, rangeDays)
		for i := 0; i < rangeDays; i++ {
			day := startDate.AddDate(0, 0, i)
			if day.After(today) {
				break
			}
			row := statsByDate[dateString(day)]
			daily = append(daily, DailyStat{
				Date:             dateString(day),
				Exercises:        row.Exercises,
				CorrectExercises: row.CorrectExercises,
				StudyMinutes:     row.StudySeconds / 60,
			})
		}
		return daily
	}

	firstMonday := startDate.AddDate(0, 0, -weekdayMondayIndex(startDate))
	daily := make([]DailyStat, 0, 53)
	for i := 0; i < 53; i++ {
		weekStart := firstMonday.AddDate(0, 0, i*7)
		if weekStart.After(today) {
			break
		}
		if weekStart.Before(startDate) {
			continue
		}
		row := statsByDate[dateString(weekStart)]
		daily = append(daily, DailyStat{
			Date:             dateString(weekStart),
			Exercises:        row.Exercises,
			CorrectExercises: row.CorrectExercises,
			StudyMinutes:     row.StudySeconds / 60,
		})
	}
	return daily
}

func buildErrorDistribution(errorCounts map[string]int) map[string]ErrorTypeDistribution {
	total := 0
	for _, count := range errorCounts {
		total += count
	}
	distribution := make(map[string]ErrorTypeDistribution, len(errorCounts))
	for key, count := range errorCounts {
		percentage := 0.0
		if total > 0 {
			percentage = round1(float64(count) / float64(total) * 100.0)
		}
		distribution[key] = ErrorTypeDistribution{Count: count, Percentage: percentage}
	}
	return distribution
}

func graphNodeType(value string) string {
	switch value {
	case "CONCEPT":
		return "concept"
	case "THEOREM":
		return "theorem"
	case "METHOD":
		return "method"
	default:
		return ""
	}
}

func graphRelationType(value string) string {
	switch value {
	case "HAS_PREREQUISITE":
		return "prerequisite"
	case "USED_IN":
		return "used_in"
	case "RELATED_TO":
		return "related"
	default:
		return ""
	}
}

func applyForgetting(mastery float64, daysSinceLast float64, floor float64) float64 {
	if floor == 0 {
		floor = dktRetentionFloor
	}
	if daysSinceLast <= 0 || mastery <= floor {
		return mastery
	}
	decayed := floor + (mastery-floor)*math.Exp(-0.05*daysSinceLast)
	return clampProbability(decayed)
}

func clampProbability(value float64) float64 {
	if value < 0.001 {
		return 0.001
	}
	if value > 0.999 {
		return 0.999
	}
	return value
}

func percent(total int, count int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) / float64(total) * 100.0
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func weekdayMondayIndex(value time.Time) int {
	weekday := int(value.Weekday())
	if weekday == 0 {
		return 6
	}
	return weekday - 1
}

func dateString(value time.Time) string {
	return value.Format("2006-01-02")
}

func normalizeTarget(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

type rankingRow struct {
	StudentID    string
	StudySeconds int
	AttemptCount int
}

type learningHeapItem struct {
	id      string
	mastery float64
}

type learningHeap []learningHeapItem

func (h learningHeap) Len() int {
	return len(h)
}

func (h learningHeap) Less(i int, j int) bool {
	if h[i].mastery == h[j].mastery {
		return h[i].id < h[j].id
	}
	return h[i].mastery < h[j].mastery
}

func (h learningHeap) Swap(i int, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *learningHeap) Push(item any) {
	*h = append(*h, item.(learningHeapItem))
}

func (h *learningHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	*h = old[:len(old)-1]
	return item
}
