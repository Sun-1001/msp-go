package exercise

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	uploadapp "mathstudy/backend-go/internal/application/upload"
	"mathstudy/backend-go/internal/platform/maputil"
	"mathstudy/backend-go/internal/platform/sliceutil"
)

// Public exercise errors mapped by the HTTP layer.
var (
	ErrNotFound   = errors.New("exercise not found")
	ErrForbidden  = errors.New("student is not enrolled")
	ErrBadRequest = errors.New("bad exercise request")
)

// Repository is the persistence surface required by exercise use cases.
type Repository interface {
	WithTx(context.Context, func(context.Context, Repository) error) error
	GetTeacherIDForStudent(context.Context, string) (string, bool, error)
	GetLatestSession(context.Context, string) (LearningSession, bool, error)
	CreateSession(context.Context, string, time.Time) (LearningSession, error)
	UpdateSessionCurrentContent(context.Context, string, *string) error
	UpdateSessionAfterSubmit(context.Context, string, []string) error
	GetExercise(context.Context, string) (Exercise, bool, error)
	ListRecentContentIDs(context.Context, string, int) ([]string, error)
	ListCandidateExercises(context.Context, CandidateFilter) ([]Exercise, error)
	GetProfile(context.Context, string) (StudentProfile, bool, error)
	HasSubmittedAttempt(context.Context, string, string) (bool, error)
	ListDKTStates(context.Context, string, []string) (map[string]DKTState, error)
	ListRecentInteractions(context.Context, string, int) ([]LearningInteraction, error)
	InsertAttempt(context.Context, AttemptRecord) error
	InsertDiagnosis(context.Context, DiagnosisRecord) error
	UpsertDKTStates(context.Context, []DKTState) error
	UpdateProfileTracking(context.Context, string, ProfileTrackingUpdate) error
}

// AnswerChecker compares student and correct answers.
type AnswerChecker interface {
	CheckAnswer(context.Context, string, string, string) (AnswerCheckResult, error)
}

// LearningSession stores the minimal session state used by exercise flow.
type LearningSession struct {
	ID                string
	StudentID         string
	CurrentContentID  *string
	ContentsAttempted []string
}

// Exercise stores problem content.
type Exercise struct {
	ID             string
	OwnerTeacherID string
	Status         string
	Title          string
	Body           string
	Difficulty     float64
	ConceptIDs     []string
	Meta           map[string]any
}

// StudentProfile stores tracking data updated after attempts.
type StudentProfile struct {
	MasteryVector       map[string]float64
	ErrorTendency       map[string]float64
	PreferredDifficulty float64
	LearningPace        float64
	TotalExercises      int
	CorrectCount        int
}

// CandidateFilter stores exercise selection filters.
type CandidateFilter struct {
	TeacherID      string
	DifficultyMin  float64
	DifficultyMax  float64
	ExcludeContent []string
	Limit          int
}

// DKTState stores one student-concept DKT state.
type DKTState struct {
	ID              string
	StudentID       string
	ConceptID       string
	MasteryProb     float64
	Confidence      float64
	AttemptCount    int
	CorrectCount    int
	IncorrectCount  int
	SequenceLength  int
	AttentionWeight float64
	LastOutcome     *bool
	LastExerciseID  *string
	LastAttemptAt   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// LearningInteraction stores one historical input event for DKT sequence updates.
type LearningInteraction struct {
	ExerciseID  string
	ConceptIDs  []string
	IsCorrect   bool
	Difficulty  float64
	SubmittedAt time.Time
}

// AttemptRecord stores data inserted into content_attempts.
type AttemptRecord struct {
	ID               string
	ContentID        string
	StudentID        string
	StudentAnswer    string
	StudentSteps     []string
	IsCorrect        bool
	Score            float64
	StartedAt        time.Time
	SubmittedAt      time.Time
	TimeSpentSeconds int
}

// DiagnosisRecord stores data inserted into diagnosis_reports.
type DiagnosisRecord struct {
	ID             string
	AttemptID      string
	ErrorType      *string
	ErrorSubtype   string
	Severity       string
	RelatedConcept []string
	Explanation    string
	Suggestion     string
	CreatedAt      time.Time
}

// ProfileTrackingUpdate stores profile updates after one attempt.
type ProfileTrackingUpdate struct {
	MasteryVector  map[string]float64
	ErrorTendency  map[string]float64
	TotalExercises int
	CorrectCount   int
	UpdatedAt      time.Time
}

// NextQuery stores /exercise/next query params.
type NextQuery struct {
	ConceptID  string
	Difficulty *float64
}

// SubmitRequest stores /exercise/submit request data.
type SubmitRequest struct {
	ExerciseID       string
	AnswerText       string
	AnswerImageURL   string
	AnswerSteps      []string
	TimeSpentSeconds int
}

// ExerciseResponse is the Python-compatible exercise summary response.
type ExerciseResponse struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Content              string   `json:"content"`
	Difficulty           float64  `json:"difficulty"`
	Type                 string   `json:"type"`
	KnowledgePoints      []string `json:"knowledge_points"`
	HintsAvailable       bool     `json:"hints_available"`
	EstimatedTimeSeconds int      `json:"estimated_time_seconds"`
	Options              []string `json:"options"`
}

// ExerciseDetailResponse is the Python-compatible exercise detail response.
type ExerciseDetailResponse struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Difficulty      float64  `json:"difficulty"`
	Type            string   `json:"type"`
	KnowledgePoints []string `json:"knowledge_points"`
	Hints           []string `json:"hints"`
	Options         []string `json:"options"`
}

// SolutionResponse is the Python-compatible solution response.
type SolutionResponse struct {
	ExerciseID string   `json:"exercise_id"`
	Answer     string   `json:"answer"`
	Steps      []string `json:"steps"`
	Source     string   `json:"source"`
}

// SubmitResponse is the Python-compatible answer submission response.
type SubmitResponse struct {
	IsCorrect          bool               `json:"is_correct"`
	Score              float64            `json:"score"`
	StudentAnswerLatex string             `json:"student_answer_latex"`
	CorrectAnswerLatex string             `json:"correct_answer_latex"`
	Diagnosis          *DiagnosisDetail   `json:"diagnosis"`
	Feedback           string             `json:"feedback"`
	MasteryUpdate      map[string]float64 `json:"mastery_update"`
	MasteryModel       string             `json:"mastery_model"`
	NextRecommendation string             `json:"next_recommendation"`
}

// DiagnosisDetail stores lightweight diagnostic feedback.
type DiagnosisDetail struct {
	ErrorType        *string  `json:"error_type"`
	ErrorSubtype     string   `json:"error_subtype,omitempty"`
	TaxonomyCode     string   `json:"taxonomy_code,omitempty"`
	ErrorDescription string   `json:"error_description"`
	ErrorStepIndex   *int     `json:"error_step_index"`
	Severity         string   `json:"severity"`
	Suggestion       string   `json:"suggestion"`
	RelatedConcepts  []string `json:"related_concepts"`
}

// AnswerCheckResult stores answer comparison output.
type AnswerCheckResult struct {
	IsCorrect  bool
	Reason     string
	Confidence float64
}

// MathSolver compares answers with an optional AI/math runtime.
type MathSolver interface {
	CheckAnswer(context.Context, AnswerCheckInput) (AnswerCheckResult, error)
}

// AnswerCheckInput carries answer comparison context into an optional solver.
type AnswerCheckInput struct {
	StudentAnswer string
	CorrectAnswer string
	AnswerType    string
	Fallback      AnswerCheckResult
}

// SolverAnswerChecker tries a configured solver before falling back to local comparison.
type SolverAnswerChecker struct {
	Solver   MathSolver
	Fallback AnswerChecker
}

// DiagnosisInput carries answer context into an optional AI diagnostician.
type DiagnosisInput struct {
	Exercise      Exercise
	StudentID     string
	StudentAnswer string
	AnswerSteps   []string
	CorrectAnswer string
	Check         AnswerCheckResult
	ImageOnly     bool
	Fallback      DiagnosisDetail
}

// Diagnostician generates structured diagnosis for incorrect attempts.
type Diagnostician interface {
	Diagnose(context.Context, DiagnosisInput) (DiagnosisDetail, error)
}

// Service implements adaptive exercise use cases.
type Service struct {
	repo          Repository
	checker       AnswerChecker
	diagnostician Diagnostician
	now           func() time.Time
	newID         func() (string, error)
}

// Option customizes the exercise service.
type Option func(*Service)

// WithDiagnostician enables AI-backed diagnosis with deterministic fallback.
func WithDiagnostician(diagnostician Diagnostician) Option {
	return func(service *Service) {
		service.diagnostician = diagnostician
	}
}

// NewService creates an exercise service.
func NewService(repo Repository, checker AnswerChecker, options ...Option) (*Service, error) {
	if repo == nil {
		return nil, errors.New("exercise repository is nil")
	}
	if checker == nil {
		checker = NormalizedAnswerChecker{}
	}
	service := &Service{
		repo:    repo,
		checker: checker,
		now:     time.Now,
		newID:   NewUUID,
	}
	for _, option := range options {
		option(service)
	}
	return service, nil
}

// GetNextExercise returns the current pending exercise or selects a new one.
func (s *Service) GetNextExercise(ctx context.Context, userID string, query NextQuery) (*ExerciseResponse, error) {
	session, err := s.getOrCreateSession(ctx, userID)
	if err != nil {
		return nil, err
	}
	if session.CurrentContentID != nil {
		current, ok, err := s.repo.GetExercise(ctx, *session.CurrentContentID)
		if err != nil {
			return nil, err
		}
		if ok && current.Status == "PUBLISHED" {
			return toExerciseResponse(current), nil
		}
		if err := s.repo.UpdateSessionCurrentContent(ctx, session.ID, nil); err != nil {
			return nil, err
		}
		session.CurrentContentID = nil
	}

	teacherID, ok, err := s.repo.GetTeacherIDForStudent(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	profile, _, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	targetConcept, targetDifficulty := chooseTarget(query, profile.MasteryVector)
	recentIDs, err := s.repo.ListRecentContentIDs(ctx, userID, 20)
	if err != nil {
		return nil, err
	}

	candidates, err := s.repo.ListCandidateExercises(ctx, CandidateFilter{
		TeacherID:      teacherID,
		DifficultyMin:  math.Max(0, targetDifficulty-0.15),
		DifficultyMax:  math.Min(1, targetDifficulty+0.15),
		ExcludeContent: recentIDs,
		Limit:          20,
	})
	if err != nil {
		return nil, err
	}
	candidates = preferConcept(candidates, targetConcept)
	if len(candidates) == 0 {
		candidates, err = s.repo.ListCandidateExercises(ctx, CandidateFilter{
			TeacherID:      teacherID,
			DifficultyMin:  0,
			DifficultyMax:  1,
			ExcludeContent: recentIDs,
			Limit:          10,
		})
		if err != nil {
			return nil, err
		}
		candidates = preferConcept(candidates, targetConcept)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	selected := selectExercise(candidates)
	currentID := selected.ID
	if err := s.repo.UpdateSessionCurrentContent(ctx, session.ID, &currentID); err != nil {
		return nil, err
	}
	return toExerciseResponse(selected), nil
}

// SubmitAnswer records an answer, performs lightweight grading, and updates tracking state.
func (s *Service) SubmitAnswer(ctx context.Context, userID string, request SubmitRequest) (SubmitResponse, error) {
	request.AnswerText = strings.TrimSpace(request.AnswerText)
	request.AnswerImageURL = strings.TrimSpace(request.AnswerImageURL)
	if request.ExerciseID == "" || (request.AnswerText == "" && request.AnswerImageURL == "") {
		return SubmitResponse{}, ErrBadRequest
	}
	if request.AnswerImageURL != "" && !isSafeAnswerImageURL(request.AnswerImageURL) {
		return SubmitResponse{}, ErrBadRequest
	}

	var response SubmitResponse
	err := s.repo.WithTx(ctx, func(txCtx context.Context, repo Repository) error {
		exercise, ok, err := repo.GetExercise(txCtx, request.ExerciseID)
		if err != nil {
			return err
		}
		if !ok || exercise.Status != "PUBLISHED" {
			return ErrBadRequest
		}
		teacherID, ok, err := repo.GetTeacherIDForStudent(txCtx, userID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrForbidden
		}
		if exercise.OwnerTeacherID != teacherID {
			return ErrBadRequest
		}

		correctAnswer := metaString(exercise.Meta, "answer")
		if correctAnswer == "" {
			return ErrBadRequest
		}

		studentAnswer := request.AnswerText
		imageOnly := false
		if studentAnswer == "" {
			imageOnly = true
			studentAnswer = request.AnswerImageURL
		}
		check := AnswerCheckResult{IsCorrect: false, Reason: "图片答案 OCR 判题能力将在 AI 迁移阶段恢复", Confidence: 0}
		if !imageOnly {
			check, err = s.checker.CheckAnswer(txCtx, studentAnswer, correctAnswer, metaString(exercise.Meta, "answer_type"))
			if err != nil {
				return err
			}
		}

		now := s.now()
		attemptID, err := s.newID()
		if err != nil {
			return err
		}
		attempt := AttemptRecord{
			ID:               attemptID,
			ContentID:        exercise.ID,
			StudentID:        userID,
			StudentAnswer:    studentAnswer,
			StudentSteps:     request.AnswerSteps,
			IsCorrect:        check.IsCorrect,
			Score:            boolScore(check.IsCorrect),
			StartedAt:        now,
			SubmittedAt:      now,
			TimeSpentSeconds: request.TimeSpentSeconds,
		}
		if err := repo.InsertAttempt(txCtx, attempt); err != nil {
			return err
		}

		var diagnosis *DiagnosisDetail
		var errorType *string
		if !check.IsCorrect {
			diagnosis = s.buildDiagnosis(txCtx, DiagnosisInput{
				Exercise:      exercise,
				StudentID:     userID,
				StudentAnswer: studentAnswer,
				AnswerSteps:   request.AnswerSteps,
				CorrectAnswer: correctAnswer,
				Check:         check,
				ImageOnly:     imageOnly,
				Fallback:      *basicDiagnosis(check.Reason, imageOnly, exercise.ConceptIDs),
			})
			errorType = diagnosis.ErrorType
			reportID, err := s.newID()
			if err != nil {
				return err
			}
			if err := repo.InsertDiagnosis(txCtx, DiagnosisRecord{
				ID:             reportID,
				AttemptID:      attempt.ID,
				ErrorType:      errorType,
				ErrorSubtype:   diagnosis.ErrorSubtype,
				Severity:       diagnosis.Severity,
				RelatedConcept: diagnosis.RelatedConcepts,
				Explanation:    diagnosis.ErrorDescription,
				Suggestion:     diagnosis.Suggestion,
				CreatedAt:      now,
			}); err != nil {
				return err
			}
		}

		session, err := s.getOrCreateSessionWithRepo(txCtx, repo, userID)
		if err != nil {
			return err
		}
		attempted := appendUnique(session.ContentsAttempted, exercise.ID)
		if err := repo.UpdateSessionAfterSubmit(txCtx, session.ID, attempted); err != nil {
			return err
		}

		masteryUpdate, err := s.updateTracking(txCtx, repo, userID, exercise, check.IsCorrect, errorType, now)
		if err != nil {
			return err
		}

		feedback := buildFeedback(check, diagnosis)
		correctAnswerLatex := ""
		if check.IsCorrect {
			correctAnswerLatex = correctAnswer
		}
		response = SubmitResponse{
			IsCorrect:          check.IsCorrect,
			Score:              attempt.Score,
			StudentAnswerLatex: studentAnswer,
			CorrectAnswerLatex: correctAnswerLatex,
			Diagnosis:          diagnosis,
			Feedback:           feedback,
			MasteryUpdate:      masteryUpdate,
			MasteryModel:       dktModelName,
			NextRecommendation: nextRecommendation(check.IsCorrect, masteryUpdate),
		}
		return nil
	})
	if err != nil {
		return SubmitResponse{}, err
	}
	return response, nil
}

func (s *Service) buildDiagnosis(ctx context.Context, input DiagnosisInput) *DiagnosisDetail {
	fallback := normalizeDiagnosis(input.Fallback, input.Exercise.ConceptIDs)
	if s.diagnostician == nil {
		return &fallback
	}
	input.Fallback = fallback
	diagnosis, err := s.diagnostician.Diagnose(ctx, input)
	if err != nil {
		return &fallback
	}
	diagnosis = normalizeDiagnosis(diagnosis, input.Exercise.ConceptIDs)
	return &diagnosis
}

// GetExercise returns exercise details when the student can access the teacher's content.
func (s *Service) GetExercise(ctx context.Context, userID string, exerciseID string) (ExerciseDetailResponse, error) {
	exercise, err := s.authorizedExercise(ctx, userID, exerciseID)
	if err != nil {
		return ExerciseDetailResponse{}, err
	}
	return ExerciseDetailResponse{
		ID:              exercise.ID,
		Title:           exercise.Title,
		Content:         exercise.Body,
		Difficulty:      exercise.Difficulty,
		Type:            metaStringDefault(exercise.Meta, "type", "short_answer"),
		KnowledgePoints: sliceutil.CloneStrings(exercise.ConceptIDs),
		Hints:           metaStringSlice(exercise.Meta, "hints"),
		Options:         metaOptionalStringSlice(exercise.Meta, "options"),
	}, nil
}

// GetSolution returns cached solution data after the student has attempted the exercise.
func (s *Service) GetSolution(ctx context.Context, userID string, exerciseID string) (SolutionResponse, error) {
	exercise, err := s.authorizedExercise(ctx, userID, exerciseID)
	if err != nil {
		return SolutionResponse{}, err
	}
	hasAttempt, err := s.repo.HasSubmittedAttempt(ctx, userID, exerciseID)
	if err != nil {
		return SolutionResponse{}, err
	}
	if !hasAttempt {
		return SolutionResponse{}, ErrNotFound
	}
	steps := metaStringSlice(exercise.Meta, "solution_steps")
	source := "unavailable"
	if len(steps) > 0 {
		source = "cached"
	}
	return SolutionResponse{
		ExerciseID: exerciseID,
		Answer:     metaString(exercise.Meta, "answer"),
		Steps:      steps,
		Source:     source,
	}, nil
}

func (s *Service) authorizedExercise(ctx context.Context, userID string, exerciseID string) (Exercise, error) {
	teacherID, ok, err := s.repo.GetTeacherIDForStudent(ctx, userID)
	if err != nil {
		return Exercise{}, err
	}
	if !ok {
		return Exercise{}, ErrForbidden
	}
	exercise, ok, err := s.repo.GetExercise(ctx, exerciseID)
	if err != nil {
		return Exercise{}, err
	}
	if !ok || exercise.OwnerTeacherID != teacherID {
		return Exercise{}, ErrNotFound
	}
	return exercise, nil
}

func (s *Service) getOrCreateSession(ctx context.Context, userID string) (LearningSession, error) {
	return s.getOrCreateSessionWithRepo(ctx, s.repo, userID)
}

func (s *Service) getOrCreateSessionWithRepo(ctx context.Context, repo Repository, userID string) (LearningSession, error) {
	session, ok, err := repo.GetLatestSession(ctx, userID)
	if err != nil {
		return LearningSession{}, err
	}
	if ok {
		return session, nil
	}
	return repo.CreateSession(ctx, userID, s.now())
}

func (s *Service) updateTracking(ctx context.Context, repo Repository, userID string, exercise Exercise, isCorrect bool, errorType *string, now time.Time) (map[string]float64, error) {
	profile, ok, err := repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	concepts := uniqueNonEmpty(exercise.ConceptIDs)
	if len(concepts) == 0 {
		return map[string]float64{}, repo.UpdateProfileTracking(ctx, userID, ProfileTrackingUpdate{
			MasteryVector:  maputil.CloneFloatMap(profile.MasteryVector),
			ErrorTendency:  maputil.CloneFloatMap(profile.ErrorTendency),
			TotalExercises: profile.TotalExercises + 1,
			CorrectCount:   profile.CorrectCount + boolInt(isCorrect),
			UpdatedAt:      now,
		})
	}

	states, err := repo.ListDKTStates(ctx, userID, concepts)
	if err != nil {
		return nil, err
	}
	history, err := repo.ListRecentInteractions(ctx, userID, dktMaxSequenceLength)
	if err != nil {
		return nil, err
	}
	current := LearningInteraction{
		ExerciseID:  exercise.ID,
		ConceptIDs:  concepts,
		IsCorrect:   isCorrect,
		Difficulty:  exercise.Difficulty,
		SubmittedAt: now,
	}
	sequence := buildDKTSequence(history, current)

	mastery := maputil.CloneFloatMap(profile.MasteryVector)
	tendency := maputil.CloneFloatMap(profile.ErrorTendency)
	if errorType != nil && !isCorrect {
		tendency[*errorType] += 1
	}
	masteryUpdate := map[string]float64{}
	upserts := make([]DKTState, 0, len(concepts))
	for _, conceptID := range concepts {
		state, hasState := states[conceptID]
		prior := mastery[conceptID]
		if prior == 0 {
			prior = dktColdStartMastery(profile.PreferredDifficulty, profile.LearningPace, exercise.Difficulty)
		}
		if hasState {
			prior = state.MasteryProb
			if state.LastAttemptAt != nil {
				daysSince := now.Sub(*state.LastAttemptAt).Hours() / 24.0
				prior = applyForgetting(prior, daysSince, dktRetentionFloor)
			}
		} else {
			id, err := s.newID()
			if err != nil {
				return nil, err
			}
			state = DKTState{
				ID:        id,
				StudentID: userID,
				ConceptID: conceptID,
				CreatedAt: now,
			}
		}
		result := dktUpdate(prior, conceptID, current, sequence, profile, optionalStringValue(errorType), state.AttemptCount)
		nextMastery := round4(result.mastery)
		nextConfidence := round4(result.confidence)
		mastery[conceptID] = nextMastery
		masteryUpdate[conceptID] = nextMastery
		outcome := isCorrect
		state.StudentID = userID
		state.ConceptID = conceptID
		state.MasteryProb = nextMastery
		state.Confidence = nextConfidence
		state.AttemptCount++
		state.CorrectCount += boolInt(isCorrect)
		state.IncorrectCount += boolInt(!isCorrect)
		state.SequenceLength = result.sequenceLength
		state.AttentionWeight = round4(result.attentionWeight)
		state.LastOutcome = &outcome
		state.LastExerciseID = &exercise.ID
		state.LastAttemptAt = &now
		state.UpdatedAt = now
		upserts = append(upserts, state)
	}
	if err := repo.UpsertDKTStates(ctx, upserts); err != nil {
		return nil, err
	}
	if err := repo.UpdateProfileTracking(ctx, userID, ProfileTrackingUpdate{
		MasteryVector:  mastery,
		ErrorTendency:  tendency,
		TotalExercises: profile.TotalExercises + 1,
		CorrectCount:   profile.CorrectCount + boolInt(isCorrect),
		UpdatedAt:      now,
	}); err != nil {
		return nil, err
	}
	return masteryUpdate, nil
}

// NormalizedAnswerChecker is a deterministic local checker used when the Math Solver agent is unavailable.
type NormalizedAnswerChecker struct{}

// CheckAnswer compares normalized strings.
func (NormalizedAnswerChecker) CheckAnswer(_ context.Context, studentAnswer string, correctAnswer string, _ string) (AnswerCheckResult, error) {
	if normalizeAnswer(studentAnswer) == normalizeAnswer(correctAnswer) {
		return AnswerCheckResult{IsCorrect: true, Reason: "答案与标准答案一致", Confidence: 1}, nil
	}
	return AnswerCheckResult{IsCorrect: false, Reason: "答案与标准答案不一致", Confidence: 0.3}, nil
}

// CheckAnswer compares answers with a solver and falls back to deterministic local comparison.
func (c SolverAnswerChecker) CheckAnswer(ctx context.Context, studentAnswer string, correctAnswer string, answerType string) (AnswerCheckResult, error) {
	fallbackChecker := c.Fallback
	if fallbackChecker == nil {
		fallbackChecker = NormalizedAnswerChecker{}
	}
	fallback, err := fallbackChecker.CheckAnswer(ctx, studentAnswer, correctAnswer, answerType)
	if err != nil {
		return AnswerCheckResult{}, err
	}
	if c.Solver == nil {
		return fallback, nil
	}
	result, err := c.Solver.CheckAnswer(ctx, AnswerCheckInput{
		StudentAnswer: studentAnswer,
		CorrectAnswer: correctAnswer,
		AnswerType:    answerType,
		Fallback:      fallback,
	})
	if err != nil {
		return fallback, nil
	}
	result.Reason = strings.TrimSpace(result.Reason)
	if result.Reason == "" {
		return fallback, nil
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		return fallback, nil
	}
	return result, nil
}

func chooseTarget(query NextQuery, mastery map[string]float64) (string, float64) {
	targetConcept := strings.TrimSpace(query.ConceptID)
	targetDifficulty := 0.5
	if query.Difficulty != nil {
		targetDifficulty = *query.Difficulty
	}
	if targetConcept != "" || query.Difficulty != nil || len(mastery) == 0 {
		return targetConcept, clamp(targetDifficulty, 0, 1)
	}
	keys := maputil.SortedFloatKeys(mastery)
	midConcept := ""
	midMastery := 0.0
	weakestConcept := ""
	weakestMastery := 1.0
	for _, concept := range keys {
		value := mastery[concept]
		if value < 0.4 && value < weakestMastery {
			weakestConcept = concept
			weakestMastery = value
		} else if value >= 0.4 && value < 0.8 && midConcept == "" {
			midConcept = concept
			midMastery = value
		}
	}
	if weakestConcept != "" {
		return weakestConcept, math.Max(0.2, weakestMastery)
	}
	if midConcept != "" {
		return midConcept, clamp(midMastery+0.1, 0, 1)
	}
	return "", targetDifficulty
}

func preferConcept(candidates []Exercise, conceptID string) []Exercise {
	if conceptID == "" || len(candidates) == 0 {
		return candidates
	}
	matched := []Exercise{}
	for _, candidate := range candidates {
		if containsString(candidate.ConceptIDs, conceptID) {
			matched = append(matched, candidate)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return candidates
}

func selectExercise(candidates []Exercise) Exercise {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0]
}

func toExerciseResponse(exercise Exercise) *ExerciseResponse {
	return &ExerciseResponse{
		ID:                   exercise.ID,
		Title:                exercise.Title,
		Content:              exercise.Body,
		Difficulty:           exercise.Difficulty,
		Type:                 metaStringDefault(exercise.Meta, "type", "short_answer"),
		KnowledgePoints:      sliceutil.CloneStrings(exercise.ConceptIDs),
		HintsAvailable:       len(metaStringSlice(exercise.Meta, "hints")) > 0,
		EstimatedTimeSeconds: metaIntDefault(exercise.Meta, "estimated_time_seconds", 300),
		Options:              metaOptionalStringSlice(exercise.Meta, "options"),
	}
}

type errorTaxonomy struct {
	ErrorType  *string
	Code       string
	Subtype    string
	Severity   string
	Suggestion string
}

func classifyMathError(reason string, imageOnly bool) errorTaxonomy {
	if imageOnly {
		errorType := "symbolic"
		return errorTaxonomy{
			ErrorType:  &errorType,
			Code:       "S-Type",
			Subtype:    "notation_or_ocr_pending",
			Severity:   "medium",
			Suggestion: "请补充可解析的文本答案，重点检查符号书写、上下标和等号链是否清晰。",
		}
	}
	normalized := strings.ToLower(reason)
	switch {
	case strings.Contains(normalized, "concept") || strings.Contains(reason, "概念") || strings.Contains(reason, "定义"):
		errorType := "conceptual"
		return errorTaxonomy{ErrorType: &errorType, Code: "C-Type", Subtype: "concept_misunderstanding", Severity: "high", Suggestion: "先回到相关定义和适用条件，再重新判断题目属于哪类对象。"}
	case strings.Contains(normalized, "logic") || strings.Contains(reason, "逻辑") || strings.Contains(reason, "充分") || strings.Contains(reason, "必要"):
		errorType := "logical"
		return errorTaxonomy{ErrorType: &errorType, Code: "L-Type", Subtype: "invalid_inference", Severity: "high", Suggestion: "逐步检查每个推理箭头是否成立，尤其区分充分条件和必要条件。"}
	case strings.Contains(normalized, "symbol") || strings.Contains(reason, "符号") || strings.Contains(reason, "格式"):
		errorType := "symbolic"
		return errorTaxonomy{ErrorType: &errorType, Code: "S-Type", Subtype: "notation_error", Severity: "medium", Suggestion: "规范书写变量、上下标、积分/微分符号和等号链，避免表达式歧义。"}
	case strings.Contains(normalized, "step") || strings.Contains(reason, "步骤") || strings.Contains(reason, "方法") || strings.Contains(reason, "过程"):
		errorType := "procedural"
		return errorTaxonomy{ErrorType: &errorType, Code: "P-Type", Subtype: "procedure_misuse", Severity: "medium", Suggestion: "对照标准算法逐步复核，先确认方法选择，再检查每一步执行。"}
	default:
		errorType := "procedural"
		return errorTaxonomy{ErrorType: &errorType, Code: "P-Type", Subtype: "answer_mismatch", Severity: "medium", Suggestion: "请按步骤复算，并标出最早与标准解不一致的位置。"}
	}
}

func basicDiagnosis(reason string, imageOnly bool, concepts []string) *DiagnosisDetail {
	explanation := "答案不正确"
	suggestion := "请重新检查解题过程"
	if reason != "" {
		explanation = reason
	}
	if imageOnly {
		suggestion = "图片答案已记录，OCR 诊断能力将在 AI 迁移阶段恢复；请优先提交文本答案以获得自动判题。"
	}
	taxonomy := classifyMathError(reason, imageOnly)
	return &DiagnosisDetail{
		ErrorType:        taxonomy.ErrorType,
		ErrorSubtype:     taxonomy.Subtype,
		TaxonomyCode:     taxonomy.Code,
		ErrorDescription: explanation,
		ErrorStepIndex:   nil,
		Severity:         taxonomy.Severity,
		Suggestion:       nonEmptyString(taxonomy.Suggestion, suggestion),
		RelatedConcepts:  sliceutil.CloneStrings(concepts),
	}
}

func normalizeDiagnosis(diagnosis DiagnosisDetail, concepts []string) DiagnosisDetail {
	if diagnosis.ErrorType != nil {
		value := strings.ToLower(strings.TrimSpace(*diagnosis.ErrorType))
		if value == "" {
			diagnosis.ErrorType = nil
		} else {
			diagnosis.ErrorType = &value
		}
	}
	diagnosis.ErrorSubtype = strings.TrimSpace(diagnosis.ErrorSubtype)
	if diagnosis.ErrorSubtype == "" {
		diagnosis.ErrorSubtype = "answer_mismatch"
	}
	diagnosis.TaxonomyCode = strings.TrimSpace(diagnosis.TaxonomyCode)
	if diagnosis.TaxonomyCode == "" && diagnosis.ErrorType != nil {
		diagnosis.TaxonomyCode = taxonomyCodeForErrorType(*diagnosis.ErrorType)
	}
	diagnosis.ErrorDescription = strings.TrimSpace(diagnosis.ErrorDescription)
	if diagnosis.ErrorDescription == "" {
		diagnosis.ErrorDescription = "答案不正确"
	}
	diagnosis.Severity = strings.ToLower(strings.TrimSpace(diagnosis.Severity))
	switch diagnosis.Severity {
	case "low", "medium", "high":
	default:
		diagnosis.Severity = "medium"
	}
	diagnosis.Suggestion = strings.TrimSpace(diagnosis.Suggestion)
	if diagnosis.Suggestion == "" {
		diagnosis.Suggestion = "请按步骤复算，并标出最早与标准解不一致的位置。"
	}
	diagnosis.RelatedConcepts = uniqueNonEmpty(append(diagnosis.RelatedConcepts, concepts...))
	return diagnosis
}

func taxonomyCodeForErrorType(errorType string) string {
	switch strings.ToLower(strings.TrimSpace(errorType)) {
	case "conceptual":
		return "C-Type"
	case "procedural":
		return "P-Type"
	case "logical":
		return "L-Type"
	case "symbolic":
		return "S-Type"
	default:
		return ""
	}
}

func buildFeedback(check AnswerCheckResult, diagnosis *DiagnosisDetail) string {
	if check.IsCorrect {
		if check.Reason == "" {
			return "回答正确！"
		}
		return "回答正确！" + check.Reason
	}
	if diagnosis != nil && diagnosis.Suggestion != "" {
		return diagnosis.Suggestion
	}
	if check.Reason != "" {
		return "答案不正确。" + check.Reason
	}
	return "答案不正确。"
}

func nextRecommendation(isCorrect bool, masteryUpdate map[string]float64) string {
	if isCorrect || len(masteryUpdate) == 0 {
		return "continue"
	}
	sum := 0.0
	for _, value := range masteryUpdate {
		sum += value
	}
	if sum/float64(len(masteryUpdate)) < 0.3 {
		return "review"
	}
	return "continue"
}

func isSafeAnswerImageURL(imageURL string) bool {
	return uploadapp.IsSafeImagePath(imageURL)
}

func normalizeAnswer(value string) string {
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}

func metaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	if value, ok := meta[key].(string); ok {
		return value
	}
	return ""
}

func metaStringDefault(meta map[string]any, key string, fallback string) string {
	value := metaString(meta, key)
	if value == "" {
		return fallback
	}
	return value
}

func metaStringSlice(meta map[string]any, key string) []string {
	values, ok := readStringSlice(meta, key)
	if !ok {
		return []string{}
	}
	return values
}

func metaOptionalStringSlice(meta map[string]any, key string) []string {
	values, ok := readStringSlice(meta, key)
	if !ok {
		return nil
	}
	return values
}

func readStringSlice(meta map[string]any, key string) ([]string, bool) {
	if meta == nil {
		return nil, false
	}
	value, exists := meta[key]
	if !exists || value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case []string:
		return sliceutil.CloneStrings(typed), true
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result, true
	default:
		return nil, false
	}
}

func metaIntDefault(meta map[string]any, key string, fallback int) int {
	if meta == nil {
		return fallback
	}
	switch value := meta[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return fallback
	}
}

func appendUnique(values []string, value string) []string {
	result := sliceutil.CloneStrings(values)
	if !containsString(result, value) {
		result = append(result, value)
	}
	return result
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nonEmptyString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolScore(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func clamp(value float64, floor float64, ceiling float64) float64 {
	if value < floor {
		return floor
	}
	if value > ceiling {
		return ceiling
	}
	return value
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
