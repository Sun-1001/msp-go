package exercise

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetNextExerciseReturnsCurrentPublishedExercise(t *testing.T) {
	currentID := "exercise-current"
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", CurrentContentID: &currentID},
		hasSession: true,
		exercises: map[string]Exercise{
			currentID: newExercise(currentID, "teacher-1", []string{"algebra"}, map[string]any{"hints": []any{"hint"}, "estimated_time_seconds": float64(180)}),
		},
	}
	service := newTestService(repo)

	response, err := service.GetNextExercise(context.Background(), "student-1", NextQuery{})
	if err != nil {
		t.Fatalf("GetNextExercise() error = %v", err)
	}
	if response == nil || response.ID != currentID || !response.HintsAvailable || response.EstimatedTimeSeconds != 180 {
		t.Fatalf("response = %#v", response)
	}
	if repo.teacherLookupCount != 0 || repo.updatedCurrent != nil {
		t.Fatalf("unexpected selection path: teacher lookups=%d updated=%#v", repo.teacherLookupCount, repo.updatedCurrent)
	}
}

func TestGetNextExerciseSelectsWeakConceptCandidate(t *testing.T) {
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1"},
		hasSession: true,
		teacherID:  "teacher-1",
		hasTeacher: true,
		recentIDs:  []string{"old"},
		profile:    StudentProfile{MasteryVector: map[string]float64{"weak": 0.25, "mid": 0.6}},
		hasProfile: true,
		candidateSet: []Exercise{
			newExercise("exercise-b", "teacher-1", []string{"mid"}, nil),
			newExercise("exercise-a", "teacher-1", []string{"weak"}, map[string]any{"type": "proof"}),
		},
	}
	service := newTestService(repo)

	response, err := service.GetNextExercise(context.Background(), "student-1", NextQuery{})
	if err != nil {
		t.Fatalf("GetNextExercise() error = %v", err)
	}
	if response == nil || response.ID != "exercise-a" || response.Type != "proof" {
		t.Fatalf("response = %#v", response)
	}
	if repo.updatedCurrent == nil || *repo.updatedCurrent != "exercise-a" {
		t.Fatalf("updated current = %#v", repo.updatedCurrent)
	}
	if len(repo.lastCandidateFilters) == 0 || repo.lastCandidateFilters[0].DifficultyMin != 0.1 || repo.lastCandidateFilters[0].DifficultyMax != 0.4 {
		t.Fatalf("candidate filters = %#v", repo.lastCandidateFilters)
	}
}

func TestSubmitAnswerRecordsCorrectAttemptAndUpdatesTracking(t *testing.T) {
	now := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1", ContentsAttempted: []string{"old"}},
		hasSession: true,
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "x+1"}),
		},
		profile: StudentProfile{
			MasteryVector:       map[string]float64{"algebra": 0.4},
			ErrorTendency:       map[string]float64{},
			PreferredDifficulty: 0.5,
			LearningPace:        1,
			TotalExercises:      2,
			CorrectCount:        1,
		},
		hasProfile: true,
	}
	service := newTestService(repo)
	service.now = func() time.Time { return now }
	service.newID = sequentialIDs("attempt-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:       "exercise-1",
		AnswerText:       " x + 1 ",
		AnswerSteps:      []string{"step"},
		TimeSpentSeconds: 60,
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if !response.IsCorrect || response.Score != 1 || response.CorrectAnswerLatex != "x+1" || response.Diagnosis != nil {
		t.Fatalf("response = %#v", response)
	}
	if len(repo.insertedAttempts) != 1 || repo.insertedAttempts[0].ID != "attempt-1" {
		t.Fatalf("attempts = %#v", repo.insertedAttempts)
	}
	if len(repo.insertedDiagnoses) != 0 {
		t.Fatalf("diagnoses = %#v", repo.insertedDiagnoses)
	}
	if response.MasteryModel != dktModelName {
		t.Fatalf("mastery model = %q", response.MasteryModel)
	}
	if len(repo.upsertedStates) != 1 || repo.upsertedStates[0].ConceptID != "algebra" || repo.upsertedStates[0].AttemptCount != 1 {
		t.Fatalf("states = %#v", repo.upsertedStates)
	}
	if repo.upsertedStates[0].SequenceLength == 0 || repo.upsertedStates[0].AttentionWeight <= 0 {
		t.Fatalf("dkt state = %#v", repo.upsertedStates[0])
	}
	if repo.profileUpdate.TotalExercises != 3 || repo.profileUpdate.CorrectCount != 2 {
		t.Fatalf("profile update = %#v", repo.profileUpdate)
	}
	if len(repo.updatedAttempted) != 2 || repo.updatedAttempted[1] != "exercise-1" {
		t.Fatalf("updated attempted = %#v", repo.updatedAttempted)
	}
}

func TestSolverAnswerCheckerUsesSolverResult(t *testing.T) {
	solver := &fakeMathSolver{result: AnswerCheckResult{IsCorrect: true, Reason: "表达式等价", Confidence: 0.92}}
	checker := SolverAnswerChecker{Solver: solver, Fallback: NormalizedAnswerChecker{}}

	result, err := checker.CheckAnswer(context.Background(), "x+x", "2x", "expression")
	if err != nil {
		t.Fatalf("CheckAnswer() error = %v", err)
	}
	if !solver.called || solver.input.StudentAnswer != "x+x" || solver.input.Fallback.IsCorrect {
		t.Fatalf("solver called=%v input=%#v", solver.called, solver.input)
	}
	if !result.IsCorrect || result.Reason != "表达式等价" || result.Confidence != 0.92 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSolverAnswerCheckerFallsBackForInvalidSolverResult(t *testing.T) {
	checker := SolverAnswerChecker{
		Solver:   &fakeMathSolver{result: AnswerCheckResult{IsCorrect: true, Reason: "", Confidence: 1.2}},
		Fallback: NormalizedAnswerChecker{},
	}

	result, err := checker.CheckAnswer(context.Background(), "x+2", "x+1", "expression")
	if err != nil {
		t.Fatalf("CheckAnswer() error = %v", err)
	}
	if result.IsCorrect || result.Reason != "答案与标准答案不一致" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSubmitAnswerRecordsBasicDiagnosisForImageOnlyAnswer(t *testing.T) {
	now := time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1"},
		hasSession: true,
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "x+1"}),
		},
		profile:    StudentProfile{MasteryVector: map[string]float64{"algebra": 0.4}, PreferredDifficulty: 0.5, LearningPace: 1},
		hasProfile: true,
	}
	service := newTestService(repo)
	service.now = func() time.Time { return now }
	service.newID = sequentialIDs("attempt-1", "diagnosis-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:     "exercise-1",
		AnswerImageURL: "/uploads/answer.png",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if response.IsCorrect || response.Diagnosis == nil {
		t.Fatalf("response = %#v", response)
	}
	if response.Diagnosis.TaxonomyCode != "S-Type" || response.Diagnosis.ErrorType == nil || *response.Diagnosis.ErrorType != "symbolic" {
		t.Fatalf("diagnosis taxonomy = %#v", response.Diagnosis)
	}
	if len(repo.insertedDiagnoses) != 1 || repo.insertedDiagnoses[0].ID != "diagnosis-1" {
		t.Fatalf("diagnoses = %#v", repo.insertedDiagnoses)
	}
	if response.NextRecommendation != "review" {
		t.Fatalf("next recommendation = %q", response.NextRecommendation)
	}
}

func TestSubmitAnswerUsesConfiguredDiagnosticianForIncorrectAnswer(t *testing.T) {
	now := time.Date(2026, time.June, 29, 10, 0, 0, 0, time.UTC)
	errorType := "conceptual"
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1"},
		hasSession: true,
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"limit"}, map[string]any{"answer": "x+1", "answer_type": "expression"}),
		},
		profile:    StudentProfile{MasteryVector: map[string]float64{"limit": 0.4}, PreferredDifficulty: 0.5, LearningPace: 1},
		hasProfile: true,
	}
	diagnostician := &fakeDiagnostician{diagnosis: DiagnosisDetail{
		ErrorType:        &errorType,
		ErrorSubtype:     "definition_confusion",
		ErrorDescription: "混淆了极限定义",
		Severity:         "high",
		Suggestion:       "先复习定义再重做。",
		RelatedConcepts:  []string{"limit"},
	}}
	service := newTestService(repo, WithDiagnostician(diagnostician))
	service.now = func() time.Time { return now }
	service.newID = sequentialIDs("attempt-1", "diagnosis-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:  "exercise-1",
		AnswerText:  "x+2",
		AnswerSteps: []string{"step 1"},
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if !diagnostician.called || diagnostician.input.CorrectAnswer != "x+1" || diagnostician.input.StudentAnswer != "x+2" {
		t.Fatalf("diagnostician called=%v input=%#v", diagnostician.called, diagnostician.input)
	}
	if response.Diagnosis == nil || response.Diagnosis.ErrorSubtype != "definition_confusion" || response.Diagnosis.TaxonomyCode != "C-Type" {
		t.Fatalf("diagnosis = %#v", response.Diagnosis)
	}
	if len(repo.insertedDiagnoses) != 1 || repo.insertedDiagnoses[0].ErrorSubtype != "definition_confusion" {
		t.Fatalf("inserted diagnoses = %#v", repo.insertedDiagnoses)
	}
}

func TestSubmitAnswerFallsBackWhenDiagnosticianFails(t *testing.T) {
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1"},
		hasSession: true,
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "x+1"}),
		},
		profile:    StudentProfile{MasteryVector: map[string]float64{"algebra": 0.4}, PreferredDifficulty: 0.5, LearningPace: 1},
		hasProfile: true,
	}
	service := newTestService(repo, WithDiagnostician(&fakeDiagnostician{err: errors.New("model unavailable")}))
	service.newID = sequentialIDs("attempt-1", "diagnosis-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID: "exercise-1",
		AnswerText: "x+2",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if response.Diagnosis == nil || response.Diagnosis.ErrorSubtype != "answer_mismatch" || response.Diagnosis.TaxonomyCode != "P-Type" {
		t.Fatalf("diagnosis = %#v", response.Diagnosis)
	}
}

func TestSubmitAnswerRejectsUnsafeImageURL(t *testing.T) {
	repo := &fakeExerciseRepo{}
	service := newTestService(repo)

	_, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:     "exercise-1",
		AnswerImageURL: "https://example.com/answer.png",
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("SubmitAnswer() error = %v, want ErrBadRequest", err)
	}
}

func TestGetSolutionRequiresSubmittedAttemptAndReturnsCachedSteps(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{
				"answer":         "42",
				"solution_steps": []any{"step 1"},
			}),
		},
		hasSubmitted: true,
	}
	service := newTestService(repo)

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if response.Answer != "42" || response.Source != "cached" || len(response.Steps) != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestGetExerciseReturnsForbiddenWhenStudentIsNotEnrolled(t *testing.T) {
	repo := &fakeExerciseRepo{hasTeacher: false}
	service := newTestService(repo)

	_, err := service.GetExercise(context.Background(), "student-1", "exercise-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetExercise() error = %v, want ErrForbidden", err)
	}
}

func newTestService(repo Repository, options ...Option) *Service {
	service, err := NewService(repo, nil, options...)
	if err != nil {
		panic(err)
	}
	service.now = func() time.Time { return time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC) }
	service.newID = sequentialIDs("id-1", "id-2", "id-3", "id-4")
	return service
}

func newExercise(id string, teacherID string, concepts []string, meta map[string]any) Exercise {
	if meta == nil {
		meta = map[string]any{}
	}
	return Exercise{
		ID:             id,
		OwnerTeacherID: teacherID,
		Status:         "PUBLISHED",
		Title:          "题目",
		Body:           "body",
		Difficulty:     0.25,
		ConceptIDs:     concepts,
		Meta:           meta,
	}
}

func sequentialIDs(values ...string) func() (string, error) {
	index := 0
	return func() (string, error) {
		if index >= len(values) {
			return "extra-id", nil
		}
		value := values[index]
		index++
		return value, nil
	}
}

type fakeDiagnostician struct {
	diagnosis DiagnosisDetail
	input     DiagnosisInput
	called    bool
	err       error
}

func (d *fakeDiagnostician) Diagnose(_ context.Context, input DiagnosisInput) (DiagnosisDetail, error) {
	d.called = true
	d.input = input
	if d.err != nil {
		return DiagnosisDetail{}, d.err
	}
	return d.diagnosis, nil
}

type fakeMathSolver struct {
	result AnswerCheckResult
	input  AnswerCheckInput
	called bool
	err    error
}

func (s *fakeMathSolver) CheckAnswer(_ context.Context, input AnswerCheckInput) (AnswerCheckResult, error) {
	s.called = true
	s.input = input
	if s.err != nil {
		return AnswerCheckResult{}, s.err
	}
	return s.result, nil
}

type fakeExerciseRepo struct {
	session              LearningSession
	hasSession           bool
	createdSession       LearningSession
	teacherID            string
	hasTeacher           bool
	teacherLookupCount   int
	exercises            map[string]Exercise
	recentIDs            []string
	candidateSet         []Exercise
	lastCandidateFilters []CandidateFilter
	profile              StudentProfile
	hasProfile           bool
	hasSubmitted         bool
	dktStates            map[string]DKTState
	interactions         []LearningInteraction
	updatedCurrent       *string
	updatedAttempted     []string
	insertedAttempts     []AttemptRecord
	insertedDiagnoses    []DiagnosisRecord
	upsertedStates       []DKTState
	profileUpdate        ProfileTrackingUpdate
}

func (r *fakeExerciseRepo) WithTx(ctx context.Context, fn func(context.Context, Repository) error) error {
	return fn(ctx, r)
}

func (r *fakeExerciseRepo) GetTeacherIDForStudent(context.Context, string) (string, bool, error) {
	r.teacherLookupCount++
	return r.teacherID, r.hasTeacher, nil
}

func (r *fakeExerciseRepo) GetLatestSession(context.Context, string) (LearningSession, bool, error) {
	return r.session, r.hasSession, nil
}

func (r *fakeExerciseRepo) CreateSession(_ context.Context, userID string, _ time.Time) (LearningSession, error) {
	r.createdSession = LearningSession{ID: "created-session", StudentID: userID}
	r.session = r.createdSession
	r.hasSession = true
	return r.createdSession, nil
}

func (r *fakeExerciseRepo) UpdateSessionCurrentContent(_ context.Context, _ string, contentID *string) error {
	if contentID == nil {
		r.updatedCurrent = nil
		return nil
	}
	value := *contentID
	r.updatedCurrent = &value
	return nil
}

func (r *fakeExerciseRepo) UpdateSessionAfterSubmit(_ context.Context, _ string, attempted []string) error {
	r.updatedAttempted = attempted
	return nil
}

func (r *fakeExerciseRepo) GetExercise(_ context.Context, exerciseID string) (Exercise, bool, error) {
	exercise, ok := r.exercises[exerciseID]
	return exercise, ok, nil
}

func (r *fakeExerciseRepo) ListRecentContentIDs(context.Context, string, int) ([]string, error) {
	return r.recentIDs, nil
}

func (r *fakeExerciseRepo) ListCandidateExercises(_ context.Context, filter CandidateFilter) ([]Exercise, error) {
	r.lastCandidateFilters = append(r.lastCandidateFilters, filter)
	return r.candidateSet, nil
}

func (r *fakeExerciseRepo) GetProfile(context.Context, string) (StudentProfile, bool, error) {
	return r.profile, r.hasProfile, nil
}

func (r *fakeExerciseRepo) HasSubmittedAttempt(context.Context, string, string) (bool, error) {
	return r.hasSubmitted, nil
}

func (r *fakeExerciseRepo) ListDKTStates(context.Context, string, []string) (map[string]DKTState, error) {
	if r.dktStates == nil {
		return map[string]DKTState{}, nil
	}
	return r.dktStates, nil
}

func (r *fakeExerciseRepo) ListRecentInteractions(context.Context, string, int) ([]LearningInteraction, error) {
	return r.interactions, nil
}

func (r *fakeExerciseRepo) InsertAttempt(_ context.Context, record AttemptRecord) error {
	r.insertedAttempts = append(r.insertedAttempts, record)
	return nil
}

func (r *fakeExerciseRepo) InsertDiagnosis(_ context.Context, record DiagnosisRecord) error {
	r.insertedDiagnoses = append(r.insertedDiagnoses, record)
	return nil
}

func (r *fakeExerciseRepo) UpsertDKTStates(_ context.Context, states []DKTState) error {
	r.upsertedStates = append(r.upsertedStates, states...)
	return nil
}

func (r *fakeExerciseRepo) UpdateProfileTracking(_ context.Context, _ string, update ProfileTrackingUpdate) error {
	r.profileUpdate = update
	return nil
}
