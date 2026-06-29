package portrait

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Repository is the persistence surface required by student portrait use cases.
type Repository interface {
	GetProfile(context.Context, string) (Profile, bool, error)
	CreateProfile(context.Context, string, time.Time) (Profile, error)
	SavePortrait(context.Context, string, string, time.Time) (Profile, bool, error)
	ClearPortrait(context.Context, string, time.Time) (bool, error)
}

// Profile stores the learning counters and generated portrait fields.
type Profile struct {
	StudentID             string
	MasteryVector         map[string]float64
	ErrorTendency         map[string]float64
	PreferredDifficulty   float64
	LearningPace          float64
	TotalExercises        int
	CorrectCount          int
	TotalStudyTimeMinutes int
	RecentConcepts        []string
	PortraitContent       *string
	PortraitGeneratedAt   *time.Time
	PortraitVersion       int
}

// PortraitResponse is the Python-compatible GET /portrait response.
type PortraitResponse struct {
	StudentID             string  `json:"student_id"`
	PortraitContent       *string `json:"portrait_content"`
	PortraitGeneratedAt   *string `json:"portrait_generated_at"`
	PortraitVersion       int     `json:"portrait_version"`
	TotalExercises        int     `json:"total_exercises"`
	CorrectRate           float64 `json:"correct_rate"`
	TotalStudyTimeMinutes int     `json:"total_study_time_minutes"`
	HasContent            bool    `json:"has_content"`
}

// GenerateResponse is the Python-compatible POST /portrait/generate response.
type GenerateResponse struct {
	PortraitContent     string `json:"portrait_content"`
	PortraitGeneratedAt string `json:"portrait_generated_at"`
	PortraitVersion     int    `json:"portrait_version"`
}

// ClearResponse is the Python-compatible DELETE /portrait response.
type ClearResponse struct {
	Cleared bool   `json:"cleared"`
	Message string `json:"message"`
}

// GeneratorInput carries normalized profile data into an optional LLM portrait generator.
type GeneratorInput struct {
	Profile         Profile
	FallbackContent string
}

// Generator creates a narrative portrait from profile data.
type Generator interface {
	GeneratePortrait(context.Context, GeneratorInput) (string, error)
}

// Service implements student portrait read and maintenance use cases.
type Service struct {
	repo      Repository
	generator Generator
	now       func() time.Time
}

// Option customizes the portrait service.
type Option func(*Service)

// WithGenerator enables configurable LLM portrait generation with template fallback.
func WithGenerator(generator Generator) Option {
	return func(service *Service) {
		service.generator = generator
	}
}

// NewService creates a portrait service.
func NewService(repo Repository, options ...Option) (*Service, error) {
	if repo == nil {
		return nil, errors.New("portrait repository is nil")
	}
	service := &Service{repo: repo, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service, nil
}

// GetPortrait returns the current student's portrait, creating an empty profile when needed.
func (s *Service) GetPortrait(ctx context.Context, userID string) (PortraitResponse, error) {
	profile, err := s.ensureProfile(ctx, userID)
	if err != nil {
		return PortraitResponse{}, err
	}
	return toPortraitResponse(profile), nil
}

// GeneratePortrait builds and stores a profile-based portrait report.
func (s *Service) GeneratePortrait(ctx context.Context, userID string) (GenerateResponse, error) {
	profile, err := s.ensureProfile(ctx, userID)
	if err != nil {
		return GenerateResponse{}, err
	}

	generatedAt := s.now()
	fallbackContent := buildPortraitContent(profile)
	content := s.generatePortraitContent(ctx, profile, fallbackContent)
	saved, ok, err := s.repo.SavePortrait(ctx, userID, content, generatedAt)
	if err != nil {
		return GenerateResponse{}, err
	}
	if !ok {
		return GenerateResponse{}, errors.New("portrait profile disappeared before save")
	}
	return GenerateResponse{
		PortraitContent:     valueOrEmpty(saved.PortraitContent),
		PortraitGeneratedAt: formatTime(generatedAt),
		PortraitVersion:     saved.PortraitVersion,
	}, nil
}

func (s *Service) generatePortraitContent(ctx context.Context, profile Profile, fallbackContent string) string {
	if s.generator == nil {
		return fallbackContent
	}
	content, err := s.generator.GeneratePortrait(ctx, GeneratorInput{
		Profile:         profile,
		FallbackContent: fallbackContent,
	})
	if err != nil {
		return fallbackContent
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return fallbackContent
	}
	return content
}

// ClearPortrait removes generated portrait content for the current student.
func (s *Service) ClearPortrait(ctx context.Context, userID string) (ClearResponse, error) {
	if _, err := s.ensureProfile(ctx, userID); err != nil {
		return ClearResponse{}, err
	}
	ok, err := s.repo.ClearPortrait(ctx, userID, s.now())
	if err != nil {
		return ClearResponse{}, err
	}
	if !ok {
		return ClearResponse{}, errors.New("portrait profile disappeared before clear")
	}
	return ClearResponse{Cleared: true, Message: "画像已清除"}, nil
}

func (s *Service) ensureProfile(ctx context.Context, userID string) (Profile, error) {
	profile, ok, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return Profile{}, err
	}
	if ok {
		return normalizeProfile(profile), nil
	}
	profile, err = s.repo.CreateProfile(ctx, userID, s.now())
	if err != nil {
		return Profile{}, err
	}
	return normalizeProfile(profile), nil
}

func toPortraitResponse(profile Profile) PortraitResponse {
	return PortraitResponse{
		StudentID:             profile.StudentID,
		PortraitContent:       profile.PortraitContent,
		PortraitGeneratedAt:   optionalFormattedTime(profile.PortraitGeneratedAt),
		PortraitVersion:       profile.PortraitVersion,
		TotalExercises:        profile.TotalExercises,
		CorrectRate:           round2(ratio(profile.CorrectCount, profile.TotalExercises)),
		TotalStudyTimeMinutes: profile.TotalStudyTimeMinutes,
		HasContent:            profile.PortraitContent != nil,
	}
}

func buildPortraitContent(profile Profile) string {
	correctRate := ratio(profile.CorrectCount, profile.TotalExercises)
	var builder strings.Builder
	builder.WriteString("# 学生画像分析报告\n\n")
	builder.WriteString("## 学习概况\n")
	builder.WriteString(fmt.Sprintf("- 总练习次数: %d\n", profile.TotalExercises))
	builder.WriteString(fmt.Sprintf("- 正确次数: %d\n", profile.CorrectCount))
	builder.WriteString(fmt.Sprintf("- 正确率: %.0f%%\n", correctRate*100))
	builder.WriteString(fmt.Sprintf("- 总学习时长: %d 分钟\n", profile.TotalStudyTimeMinutes))
	builder.WriteString(fmt.Sprintf("- 偏好难度: %.2f\n", profile.PreferredDifficulty))
	builder.WriteString(fmt.Sprintf("- 学习节奏系数: %.2f\n", profile.LearningPace))

	if len(profile.MasteryVector) > 0 {
		builder.WriteString("\n## 知识点掌握度\n")
		for _, item := range sortedTop(profile.MasteryVector, true, 10) {
			builder.WriteString(fmt.Sprintf("- %s: %.0f%%\n", item.key, item.value*100))
		}
	}

	if len(profile.ErrorTendency) > 0 {
		builder.WriteString("\n## 错误倾向\n")
		for _, item := range sortedTop(profile.ErrorTendency, false, 8) {
			builder.WriteString(fmt.Sprintf("- %s: %s 次\n", item.key, formatNumber(item.value)))
		}
	}

	if len(profile.RecentConcepts) > 0 {
		builder.WriteString("\n## 近期学习重点\n")
		for _, concept := range profile.RecentConcepts {
			builder.WriteString(fmt.Sprintf("- %s\n", concept))
		}
	}

	builder.WriteString("\n## 改进建议\n")
	if profile.TotalExercises == 0 {
		builder.WriteString("- 先完成一组基础练习，积累可分析的学习记录。\n")
	} else if correctRate < 0.6 {
		builder.WriteString("- 优先复盘近期错题，针对低掌握知识点进行小步练习。\n")
	} else if correctRate < 0.85 {
		builder.WriteString("- 保持当前节奏，增加中等难度题目的稳定训练。\n")
	} else {
		builder.WriteString("- 可以提高题目难度，并开始总结解题方法迁移到综合题。\n")
	}
	return builder.String()
}

func normalizeProfile(profile Profile) Profile {
	if profile.MasteryVector == nil {
		profile.MasteryVector = map[string]float64{}
	}
	if profile.ErrorTendency == nil {
		profile.ErrorTendency = map[string]float64{}
	}
	if profile.RecentConcepts == nil {
		profile.RecentConcepts = []string{}
	}
	return profile
}

func optionalFormattedTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.Format("2006-01-02T15:04:05.999999")
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func ratio(count int, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) / float64(total)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func formatNumber(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.000001 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.2f", value)
}

type scoreItem struct {
	key   string
	value float64
}

func sortedTop(values map[string]float64, ascending bool, limit int) []scoreItem {
	items := make([]scoreItem, 0, len(values))
	for key, value := range values {
		items = append(items, scoreItem{key: key, value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].value == items[j].value {
			return items[i].key < items[j].key
		}
		if ascending {
			return items[i].value < items[j].value
		}
		return items[i].value > items[j].value
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}
