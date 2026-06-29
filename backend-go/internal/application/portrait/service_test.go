package portrait

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGetPortraitCreatesProfileAndComputesCorrectRate(t *testing.T) {
	generatedAt := time.Date(2026, time.April, 25, 10, 30, 0, 0, time.UTC)
	content := "existing portrait"
	repo := &fakePortraitRepo{
		hasProfile: false,
		createdProfile: Profile{
			StudentID:             "student-1",
			TotalExercises:        4,
			CorrectCount:          3,
			TotalStudyTimeMinutes: 90,
			PortraitContent:       &content,
			PortraitGeneratedAt:   &generatedAt,
			PortraitVersion:       2,
		},
	}
	service := newTestService(repo, generatedAt)

	response, err := service.GetPortrait(context.Background(), "student-1")
	if err != nil {
		t.Fatalf("GetPortrait() error = %v", err)
	}
	if repo.createdUserID != "student-1" {
		t.Fatalf("createdUserID = %q", repo.createdUserID)
	}
	if response.CorrectRate != 0.75 || response.TotalStudyTimeMinutes != 90 {
		t.Fatalf("response counters = %#v", response)
	}
	if !response.HasContent || response.PortraitContent == nil || *response.PortraitContent != content {
		t.Fatalf("portrait content = %#v", response)
	}
	if response.PortraitGeneratedAt == nil || *response.PortraitGeneratedAt == "" {
		t.Fatalf("generated at = %#v", response.PortraitGeneratedAt)
	}
}

func TestGeneratePortraitBuildsReportAndStoresVersion(t *testing.T) {
	now := time.Date(2026, time.April, 25, 11, 0, 0, 0, time.UTC)
	repo := &fakePortraitRepo{
		hasProfile: true,
		profile: Profile{
			StudentID:             "student-1",
			MasteryVector:         map[string]float64{"极限": 0.4, "导数": 0.8},
			ErrorTendency:         map[string]float64{"conceptual": 3},
			PreferredDifficulty:   0.6,
			LearningPace:          1.2,
			TotalExercises:        10,
			CorrectCount:          8,
			TotalStudyTimeMinutes: 120,
			RecentConcepts:        []string{"极限", "导数"},
			PortraitVersion:       1,
		},
	}
	service := newTestService(repo, now)

	response, err := service.GeneratePortrait(context.Background(), "student-1")
	if err != nil {
		t.Fatalf("GeneratePortrait() error = %v", err)
	}
	if repo.savedUserID != "student-1" || repo.savedAt != now {
		t.Fatalf("save inputs = user %q at %v", repo.savedUserID, repo.savedAt)
	}
	if !strings.Contains(repo.savedContent, "学习概况") || !strings.Contains(repo.savedContent, "改进建议") {
		t.Fatalf("saved content = %q", repo.savedContent)
	}
	if response.PortraitVersion != 2 || response.PortraitContent == "" || response.PortraitGeneratedAt == "" {
		t.Fatalf("response = %#v", response)
	}
}

func TestGeneratePortraitUsesConfiguredGenerator(t *testing.T) {
	now := time.Date(2026, time.June, 29, 9, 0, 0, 0, time.UTC)
	repo := &fakePortraitRepo{
		hasProfile: true,
		profile: Profile{
			StudentID:      "student-1",
			TotalExercises: 6,
			CorrectCount:   4,
		},
	}
	generator := &fakeGenerator{content: "# LLM 画像\n\n建议保持练习节奏。"}
	service := newTestService(repo, now, WithGenerator(generator))

	response, err := service.GeneratePortrait(context.Background(), "student-1")
	if err != nil {
		t.Fatalf("GeneratePortrait() error = %v", err)
	}
	if !generator.called || generator.input.Profile.StudentID != "student-1" {
		t.Fatalf("generator called=%v input=%#v", generator.called, generator.input)
	}
	if !strings.Contains(generator.input.FallbackContent, "学习概况") {
		t.Fatalf("fallback content = %q", generator.input.FallbackContent)
	}
	if repo.savedContent != generator.content || response.PortraitContent != generator.content {
		t.Fatalf("saved=%q response=%#v", repo.savedContent, response)
	}
}

func TestGeneratePortraitFallsBackWhenGeneratorFails(t *testing.T) {
	now := time.Date(2026, time.June, 29, 9, 30, 0, 0, time.UTC)
	repo := &fakePortraitRepo{
		hasProfile: true,
		profile: Profile{
			StudentID:      "student-1",
			TotalExercises: 2,
			CorrectCount:   1,
		},
	}
	service := newTestService(repo, now, WithGenerator(&fakeGenerator{err: errors.New("model unavailable")}))

	if _, err := service.GeneratePortrait(context.Background(), "student-1"); err != nil {
		t.Fatalf("GeneratePortrait() error = %v", err)
	}
	if !strings.Contains(repo.savedContent, "学习概况") || !strings.Contains(repo.savedContent, "改进建议") {
		t.Fatalf("saved content = %q", repo.savedContent)
	}
}

func TestClearPortraitEnsuresProfileAndReturnsPythonMessage(t *testing.T) {
	now := time.Date(2026, time.April, 25, 12, 0, 0, 0, time.UTC)
	repo := &fakePortraitRepo{
		hasProfile: true,
		profile:    Profile{StudentID: "student-1", PortraitVersion: 3},
	}
	service := newTestService(repo, now)

	response, err := service.ClearPortrait(context.Background(), "student-1")
	if err != nil {
		t.Fatalf("ClearPortrait() error = %v", err)
	}
	if repo.clearedUserID != "student-1" || repo.clearedAt != now {
		t.Fatalf("clear inputs = user %q at %v", repo.clearedUserID, repo.clearedAt)
	}
	if !response.Cleared || response.Message != "画像已清除" {
		t.Fatalf("response = %#v", response)
	}
}

func TestNewServiceRejectsMissingRepository(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) error = nil, want error")
	}
}

func newTestService(repo Repository, now time.Time, options ...Option) *Service {
	service, err := NewService(repo, options...)
	if err != nil {
		panic(err)
	}
	service.now = func() time.Time { return now }
	return service
}

type fakeGenerator struct {
	content string
	input   GeneratorInput
	called  bool
	err     error
}

func (g *fakeGenerator) GeneratePortrait(_ context.Context, input GeneratorInput) (string, error) {
	g.called = true
	g.input = input
	if g.err != nil {
		return "", g.err
	}
	return g.content, nil
}

type fakePortraitRepo struct {
	profile        Profile
	hasProfile     bool
	createdProfile Profile

	createdUserID string
	savedUserID   string
	savedContent  string
	savedAt       time.Time
	clearedUserID string
	clearedAt     time.Time
}

func (r *fakePortraitRepo) GetProfile(context.Context, string) (Profile, bool, error) {
	return r.profile, r.hasProfile, nil
}

func (r *fakePortraitRepo) CreateProfile(_ context.Context, userID string, _ time.Time) (Profile, error) {
	r.createdUserID = userID
	if r.createdProfile.StudentID == "" {
		r.createdProfile.StudentID = userID
	}
	return r.createdProfile, nil
}

func (r *fakePortraitRepo) SavePortrait(_ context.Context, userID string, content string, generatedAt time.Time) (Profile, bool, error) {
	r.savedUserID = userID
	r.savedContent = content
	r.savedAt = generatedAt
	saved := r.profile
	saved.PortraitContent = &content
	saved.PortraitGeneratedAt = &generatedAt
	saved.PortraitVersion++
	return saved, true, nil
}

func (r *fakePortraitRepo) ClearPortrait(_ context.Context, userID string, updatedAt time.Time) (bool, error) {
	r.clearedUserID = userID
	r.clearedAt = updatedAt
	return true, nil
}
