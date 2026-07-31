package progress

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"mathstudy/backend/internal/application/learningrange"
	"mathstudy/backend/internal/platform/timefmt"
)

var (
	// ErrInvalidPortraitAction is returned when an action concept is empty.
	ErrInvalidPortraitAction = errors.New("invalid portrait action")
	// ErrPortraitActionConceptNotFound is returned when an action concept does not exist.
	ErrPortraitActionConceptNotFound = errors.New("portrait action concept not found")
)

// PortraitActionProgress is a student-started action with DKT-derived progress.
type PortraitActionProgress struct {
	ConceptID      string
	TargetCount    int
	CompletedCount int
	StartedAt      time.Time
}

// PortraitAction gives the frontend a concrete destination instead of a prose-only suggestion.
type PortraitAction struct {
	Type           string  `json:"type"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	ConceptID      *string `json:"concept_id"`
	TargetCount    int     `json:"target_count"`
	CompletedCount int     `json:"completed_count"`
	Status         string  `json:"status"`
	StartedAt      *string `json:"started_at"`
}

// PortraitActionStartResponse confirms an explicit student action start.
type PortraitActionStartResponse struct {
	ConceptID      string `json:"concept_id"`
	TargetCount    int    `json:"target_count"`
	CompletedCount int    `json:"completed_count"`
	Status         string `json:"status"`
	StartedAt      string `json:"started_at"`
}

// StartPortraitAction starts an explicit practice plan without coupling exercise writes to portraits.
func (s *Service) StartPortraitAction(ctx context.Context, userID string, conceptID string) (PortraitActionStartResponse, error) {
	conceptID = strings.TrimSpace(conceptID)
	if conceptID == "" {
		return PortraitActionStartResponse{}, ErrInvalidPortraitAction
	}
	startedAt := s.now().UTC()
	found, err := s.repo.StartPortraitAction(ctx, userID, conceptID, startedAt)
	if err != nil {
		return PortraitActionStartResponse{}, err
	}
	if !found {
		return PortraitActionStartResponse{}, ErrPortraitActionConceptNotFound
	}
	progresses, err := s.repo.ListPortraitActionProgresses(ctx, userID)
	if err != nil {
		return PortraitActionStartResponse{}, err
	}
	progress, ok := progresses[conceptID]
	if !ok {
		return PortraitActionStartResponse{}, errors.New("portrait action disappeared after start")
	}
	status := "in_progress"
	if progress.CompletedCount >= progress.TargetCount {
		status = "completed"
	}
	return PortraitActionStartResponse{
		ConceptID:      conceptID,
		TargetCount:    progress.TargetCount,
		CompletedCount: min(progress.CompletedCount, progress.TargetCount),
		Status:         status,
		StartedAt:      timefmt.DateTimeRFC3339(learningrange.InPlatformZone(progress.StartedAt)),
	}, nil
}

func portraitActions(improvements []PortraitTopic, progresses map[string]PortraitActionProgress, names map[string]string) []PortraitAction {
	actions := make([]PortraitAction, 0, 3)
	included := make(map[string]struct{}, 3)
	active := make([]PortraitActionProgress, 0, len(progresses))
	for _, progress := range progresses {
		if progress.CompletedCount < progress.TargetCount {
			active = append(active, progress)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].StartedAt.Equal(active[j].StartedAt) {
			return active[i].ConceptID < active[j].ConceptID
		}
		return active[i].StartedAt.Before(active[j].StartedAt)
	})
	for _, progress := range active {
		if len(actions) >= 3 {
			break
		}
		name := strings.TrimSpace(names[progress.ConceptID])
		if name == "" {
			name = "该知识点"
		}
		actions = append(actions, portraitPracticeAction(progress.ConceptID, name, &progress))
		included[progress.ConceptID] = struct{}{}
	}
	for _, topic := range improvements {
		if len(actions) >= 3 {
			break
		}
		if _, ok := included[topic.ConceptID]; ok {
			continue
		}
		progress, ok := progresses[topic.ConceptID]
		if ok {
			actions = append(actions, portraitPracticeAction(topic.ConceptID, topic.Name, &progress))
		} else {
			actions = append(actions, portraitPracticeAction(topic.ConceptID, topic.Name, nil))
		}
		included[topic.ConceptID] = struct{}{}
	}
	if len(actions) == 0 {
		actions = append(actions, PortraitAction{
			Type:        "review",
			Title:       "复习近期错题",
			Description: "重做近期错题，检查已经掌握的知识是否保持稳定。",
			Status:      "not_started",
		})
	}
	return actions
}

func portraitPracticeAction(conceptID string, name string, progress *PortraitActionProgress) PortraitAction {
	target := 10
	completed := 0
	status := "not_started"
	var startedAt *string
	if progress != nil {
		target = progress.TargetCount
		completed = min(progress.CompletedCount, target)
		status = "in_progress"
		if completed >= target {
			status = "completed"
		}
		formatted := timefmt.DateTimeRFC3339(learningrange.InPlatformZone(progress.StartedAt))
		startedAt = &formatted
	}
	return PortraitAction{
		Type:           "practice",
		Title:          "巩固" + name,
		Description:    "完成10道针对性练习，并优先复盘错误步骤。",
		ConceptID:      &conceptID,
		TargetCount:    target,
		CompletedCount: completed,
		Status:         status,
		StartedAt:      startedAt,
	}
}
