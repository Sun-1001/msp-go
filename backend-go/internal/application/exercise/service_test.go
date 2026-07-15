package exercise

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	answerocrapp "mathstudy/backend-go/internal/application/answerocr"
	mathsolverapp "mathstudy/backend-go/internal/application/mathsolver"
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

func TestGenerateExercisePersistsStudentOwnedQuestion(t *testing.T) {
	repo := &fakeExerciseRepo{
		knowledgeConcept:    KnowledgeConcept{ID: "limit", Name: "函数极限", Description: "研究函数趋近行为", Chapter: "第一章"},
		hasKnowledgeConcept: true,
	}
	generator := &fakeQuestionGenerator{question: GeneratedQuestion{
		Title:                "函数极限判断",
		Body:                 "下列关于函数极限的说法正确的是？",
		Type:                 "multiple_choice",
		Answer:               " 极限描述函数在某点附近的趋近行为 ",
		AnswerType:           "text",
		Options:              []string{"极限只与函数在该点的值有关", "极限描述函数在某点附近的趋近行为", "所有函数处处存在极限", "极限只能是整数"},
		Hints:                []string{"区分函数值与邻域内的变化趋势。"},
		SolutionSteps:        []string{"根据极限定义判断每个选项。"},
		EstimatedTimeSeconds: 120,
	}}
	service := newTestService(repo, WithQuestionGenerator(generator))

	response, err := service.GenerateExercise(context.Background(), "student-1", GenerateExerciseRequest{ConceptID: " limit ", Difficulty: 0.5})
	if err != nil {
		t.Fatalf("GenerateExercise() error = %v", err)
	}
	if response == nil || response.Source != ExerciseSourceAIGenerated || response.Difficulty != 0.5 || response.ID != "generated-1" {
		t.Fatalf("response = %#v", response)
	}
	if !generator.called || generator.input.Concept.ID != "limit" || generator.input.Difficulty != 0.5 {
		t.Fatalf("generator = %#v", generator)
	}
	if repo.createdGeneratedStudentID != "student-1" || repo.createdGenerated.Difficulty != 0.5 || repo.createdGenerated.AnswerType != "text" ||
		repo.createdGenerated.Answer != "极限描述函数在某点附近的趋近行为" {
		t.Fatalf("persisted generated question = %#v student=%q", repo.createdGenerated, repo.createdGeneratedStudentID)
	}
	if len(repo.createdGenerated.ConceptIDs) != 1 || repo.createdGenerated.ConceptIDs[0] != "limit" || repo.createdGenerated.KnowledgePointNames[0] != "函数极限" {
		t.Fatalf("generated concept context = %#v", repo.createdGenerated)
	}
}

func TestGenerateExerciseFailsClosedWhenGeneratorFails(t *testing.T) {
	repo := &fakeExerciseRepo{
		knowledgeConcept:    KnowledgeConcept{ID: "limit", Name: "函数极限"},
		hasKnowledgeConcept: true,
	}
	service := newTestService(repo, WithQuestionGenerator(&fakeQuestionGenerator{err: errors.New("model unavailable")}))

	_, err := service.GenerateExercise(context.Background(), "student-1", GenerateExerciseRequest{ConceptID: "limit", Difficulty: 0.5})
	if !errors.Is(err, ErrAIGenerationUnavailable) {
		t.Fatalf("GenerateExercise() error = %v, want ErrAIGenerationUnavailable", err)
	}
	if repo.createdGeneratedStudentID != "" {
		t.Fatalf("invalid generation was persisted: %#v", repo.createdGenerated)
	}
}

func TestGenerateExerciseRejectsInvalidStructuredQuestion(t *testing.T) {
	repo := &fakeExerciseRepo{
		knowledgeConcept:    KnowledgeConcept{ID: "limit", Name: "函数极限"},
		hasKnowledgeConcept: true,
	}
	service := newTestService(repo, WithQuestionGenerator(&fakeQuestionGenerator{question: GeneratedQuestion{
		Title: "题目", Body: "题干", Type: "multiple_choice", Answer: "A", AnswerType: "text",
		Options: []string{"A", "A", "B", "C"}, Hints: []string{"提示"}, SolutionSteps: []string{"解析"}, EstimatedTimeSeconds: 60,
	}}))

	_, err := service.GenerateExercise(context.Background(), "student-1", GenerateExerciseRequest{ConceptID: "limit", Difficulty: 0.5})
	if !errors.Is(err, ErrAIGenerationUnavailable) {
		t.Fatalf("GenerateExercise() error = %v, want ErrAIGenerationUnavailable", err)
	}
	if repo.createdGeneratedStudentID != "" {
		t.Fatal("invalid generated question was persisted")
	}
}

func TestGenerateExerciseRejectsAnswerOutsideOptions(t *testing.T) {
	repo := &fakeExerciseRepo{
		knowledgeConcept:    KnowledgeConcept{ID: "limit", Name: "函数极限"},
		hasKnowledgeConcept: true,
	}
	service := newTestService(repo, WithQuestionGenerator(&fakeQuestionGenerator{question: GeneratedQuestion{
		Title: "题目", Body: "题干", Type: "multiple_choice", Answer: "E", AnswerType: "text",
		Options: []string{"A1", "B1", "C1", "D1"}, Hints: []string{"提示"}, SolutionSteps: []string{"解析"}, EstimatedTimeSeconds: 60,
	}}))

	_, err := service.GenerateExercise(context.Background(), "student-1", GenerateExerciseRequest{ConceptID: "limit", Difficulty: 0.5})
	if !errors.Is(err, ErrAIGenerationUnavailable) {
		t.Fatalf("GenerateExercise() error = %v, want ErrAIGenerationUnavailable", err)
	}
	if repo.createdGeneratedStudentID != "" {
		t.Fatal("invalid generated question was persisted")
	}
}

func TestGenerateExerciseRejectsMissingKnowledgeConcept(t *testing.T) {
	service := newTestService(&fakeExerciseRepo{}, WithQuestionGenerator(&fakeQuestionGenerator{}))

	_, err := service.GenerateExercise(context.Background(), "student-1", GenerateExerciseRequest{ConceptID: "missing", Difficulty: 0.5})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GenerateExercise() error = %v, want ErrNotFound", err)
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
		AnswerImageURL:   "/uploads/images/supporting-work.png",
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
	if repo.insertedAttempts[0].StudentAnswer != "x + 1" {
		t.Fatalf("student answer = %q", repo.insertedAttempts[0].StudentAnswer)
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
	if repo.createdProfileUserID != "" {
		t.Fatalf("existing profile was recreated for %q", repo.createdProfileUserID)
	}
	if len(repo.updatedAttempted) != 2 || repo.updatedAttempted[1] != "exercise-1" {
		t.Fatalf("updated attempted = %#v", repo.updatedAttempted)
	}
	if repo.exerciseForUpdateCount != 1 {
		t.Fatalf("transactional exercise lock reads = %d, want 1", repo.exerciseForUpdateCount)
	}
}

func TestSubmitAnswerAllowsOwnGeneratedQuestionWithoutClass(t *testing.T) {
	pendingClassExerciseID := "class-pending"
	repo := &fakeExerciseRepo{
		session: LearningSession{
			ID:               "session-1",
			StudentID:        "student-1",
			CurrentContentID: &pendingClassExerciseID,
		},
		hasSession: true,
		exercises: map[string]Exercise{
			"generated-1": {
				ID: "generated-1", GeneratedByStudentID: "student-1", Status: "PUBLISHED", Title: "AI题", Body: "题干", Difficulty: 0.5,
				ConceptIDs: []string{"limit"}, Meta: map[string]any{"answer": "B", "answer_type": "text"},
			},
		},
	}
	service := newTestService(repo)
	service.newID = sequentialIDs("attempt-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{ExerciseID: "generated-1", AnswerText: "B"})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if !response.IsCorrect || repo.teacherLookupCount != 0 || len(repo.insertedAttempts) != 1 {
		t.Fatalf("response=%#v repo=%#v", response, repo)
	}
	if repo.createdProfileUserID != "student-1" || len(repo.upsertedStates) != 1 || repo.upsertedStates[0].ConceptID != "limit" {
		t.Fatalf("profile user=%q states=%#v", repo.createdProfileUserID, repo.upsertedStates)
	}
	if repo.profileUpdate.TotalExercises != 1 || repo.profileUpdate.CorrectCount != 1 || response.MasteryUpdate["limit"] == 0 {
		t.Fatalf("profile update=%#v mastery=%#v", repo.profileUpdate, response.MasteryUpdate)
	}
	if repo.updatedAfterSubmitExerciseID != "generated-1" {
		t.Fatalf("submitted exercise id = %q", repo.updatedAfterSubmitExerciseID)
	}
}

func TestSubmitAnswerAcceptsClassOptionTextForLetterAnswer(t *testing.T) {
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1"},
		hasSession: true,
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"class-choice": newExercise("class-choice", "teacher-1", []string{"limit"}, map[string]any{
				"type":        "multiple_choice",
				"answer":      "A",
				"answer_type": "text",
				"options":     []any{"函数在该点附近趋近同一数值", "函数值必须等于极限", "极限只能是整数", "所有函数都有极限"},
			}),
		},
		profile: StudentProfile{
			MasteryVector:       map[string]float64{"limit": 0.4},
			ErrorTendency:       map[string]float64{},
			PreferredDifficulty: 0.5,
			LearningPace:        1,
		},
		hasProfile: true,
	}
	service := newTestService(repo)
	service.newID = sequentialIDs("attempt-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID: "class-choice",
		AnswerText: "函数在该点附近趋近同一数值",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if !response.IsCorrect || len(repo.insertedAttempts) != 1 || !repo.insertedAttempts[0].IsCorrect {
		t.Fatalf("response=%#v attempts=%#v", response, repo.insertedAttempts)
	}
}

func TestDeterministicMultipleChoiceCheckSupportsLabelsAndOptionText(t *testing.T) {
	options := []any{"B", "A", "4", "8"}
	tests := []struct {
		name       string
		exercise   Exercise
		student    string
		correct    string
		wantResult bool
	}{
		{
			name:       "class label maps before ambiguous option text",
			exercise:   Exercise{Meta: map[string]any{"type": "multiple_choice", "options": options}},
			student:    "B",
			correct:    "A",
			wantResult: true,
		},
		{
			name:       "class text answer accepts option label",
			exercise:   Exercise{Meta: map[string]any{"type": "multiple_choice", "options": options}},
			student:    "c",
			correct:    "4",
			wantResult: true,
		},
		{
			name:       "AI answer text wins over label ambiguity",
			exercise:   Exercise{GeneratedByStudentID: "student-1", Meta: map[string]any{"type": "multiple_choice", "options": options}},
			student:    "A",
			correct:    "A",
			wantResult: true,
		},
		{
			name:       "AI text answer also accepts label client",
			exercise:   Exercise{GeneratedByStudentID: "student-1", Meta: map[string]any{"type": "multiple_choice", "options": options}},
			student:    "C",
			correct:    "4",
			wantResult: true,
		},
		{
			name:       "different option remains incorrect",
			exercise:   Exercise{Meta: map[string]any{"type": "multiple_choice", "options": options}},
			student:    "D",
			correct:    "C",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := deterministicMultipleChoiceCheck(tt.exercise, tt.student, tt.correct)
			if !ok || result.IsCorrect != tt.wantResult || result.Confidence != 1 {
				t.Fatalf("deterministicMultipleChoiceCheck() = %#v, %t", result, ok)
			}
		})
	}
}

func TestSubmitAnswerRejectsInvalidMultipleChoiceReferenceBeforeLearningWrites(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID: "teacher-1", hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{
				"type": "multiple_choice", "answer_type": "text", "answer": "E",
				"options": []any{"A1", "B1", "C1", "D1"},
			}),
		},
	}
	checker := &recordingAnswerChecker{result: AnswerCheckResult{IsCorrect: true, Reason: "must not run", Confidence: 1}}
	service, err := NewService(repo, checker)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{ExerciseID: "exercise-1", AnswerText: "E"})
	if !errors.Is(err, ErrAnswerParseFailed) {
		t.Fatalf("SubmitAnswer() error = %v, want ErrAnswerParseFailed", err)
	}
	if checker.called {
		t.Fatal("general checker was called for an invalid multiple-choice reference")
	}
	assertNoSubmissionMutation(t, repo)
}

func TestSubmitAnswerReturnsProfileCreationErrorBeforeDKTUpdate(t *testing.T) {
	wantErr := errors.New("create profile failed")
	repo := &fakeExerciseRepo{
		session:          LearningSession{ID: "session-1", StudentID: "student-1"},
		hasSession:       true,
		createProfileErr: wantErr,
		exercises: map[string]Exercise{
			"generated-1": {
				ID:                   "generated-1",
				GeneratedByStudentID: "student-1",
				Status:               "PUBLISHED",
				ConceptIDs:           []string{"limit"},
				Meta:                 map[string]any{"answer": "B", "answer_type": "text"},
			},
		},
	}
	service := newTestService(repo)
	service.newID = sequentialIDs("attempt-1")

	_, err := service.SubmitAnswer(
		context.Background(),
		"student-1",
		SubmitRequest{ExerciseID: "generated-1", AnswerText: "B"},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("SubmitAnswer() error = %v, want %v", err, wantErr)
	}
	if len(repo.upsertedStates) != 0 || repo.profileUpdate.TotalExercises != 0 {
		t.Fatalf("tracking continued after profile failure: states=%#v profile=%#v", repo.upsertedStates, repo.profileUpdate)
	}
}

func TestCheckExerciseAnswerFailsClosedWithoutStructuredOptions(t *testing.T) {
	checker := &recordingAnswerChecker{result: AnswerCheckResult{IsCorrect: true, Reason: "fallback", Confidence: 0.8}}
	service, err := NewService(&fakeExerciseRepo{}, checker)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.checkExerciseAnswer(
		context.Background(),
		Exercise{Meta: map[string]any{"type": "multiple_choice", "answer_type": "text"}},
		"A",
		"A",
	)
	if err != nil {
		t.Fatalf("checkExerciseAnswer() error = %v", err)
	}
	if checker.called || result.Decision != mathsolverapp.DecisionIndeterminate || result.Failure == nil ||
		result.Failure.Code != mathsolverapp.FailureInvalidInput || result.Failure.Stage != "reference_answer" {
		t.Fatalf("checker called=%t result=%#v", checker.called, result)
	}
}

func TestGetExerciseHidesAnotherStudentsGeneratedQuestion(t *testing.T) {
	repo := &fakeExerciseRepo{exercises: map[string]Exercise{
		"generated-1": {ID: "generated-1", GeneratedByStudentID: "student-2", Status: "PUBLISHED", Meta: map[string]any{}},
	}}
	service := newTestService(repo)

	_, err := service.GetExercise(context.Background(), "student-1", "generated-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetExercise() error = %v, want ErrNotFound", err)
	}
	if repo.teacherLookupCount != 0 {
		t.Fatalf("generated access unexpectedly queried class teacher %d times", repo.teacherLookupCount)
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
	if result.IsCorrect || result.Decision != mathsolverapp.DecisionIndeterminate || result.ReasonCode != "invalid_confidence" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSolverAnswerCheckerPreservesCancellationAsNonRetryable(t *testing.T) {
	checker := SolverAnswerChecker{
		Solver:   &fakeMathSolver{err: context.Canceled},
		Fallback: NormalizedAnswerChecker{},
	}

	result, err := checker.CheckAnswer(context.Background(), "x+x", "2*x", "expression")
	if err != nil {
		t.Fatalf("CheckAnswer() error = %v", err)
	}
	if result.Decision != mathsolverapp.DecisionIndeterminate || result.Retryable || result.Failure == nil ||
		result.Failure.Code != mathsolverapp.FailureCanceled || result.Failure.Retryable ||
		result.ReasonCode != "solver_canceled" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSubmitAnswerRejectsImageOnlyBeforeTransaction(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "x+1", "answer_type": "expression"}),
		},
	}
	service := newTestService(repo)

	_, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:     "exercise-1",
		AnswerImageURL: "/uploads/images/answer.png",
	})
	if !errors.Is(err, ErrOCRUnavailable) {
		t.Fatalf("SubmitAnswer() error = %v, want ErrOCRUnavailable", err)
	}
	if repo.withTxCalled || len(repo.insertedAttempts) != 0 || len(repo.insertedDiagnoses) != 0 || len(repo.upsertedStates) != 0 || repo.createdSession.StudentID != "" || repo.createdProfileUserID != "" || repo.profileUpdate.TotalExercises != 0 {
		t.Fatalf("image-only answer reached persistence: %#v", repo)
	}
}

func TestSubmitAnswerRecognizesImageBeforeTransactionAndPersistsRecognizedAnswer(t *testing.T) {
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1"},
		hasSession: true,
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "x+1", "answer_type": "expression"}),
		},
		profile:    StudentProfile{MasteryVector: map[string]float64{"algebra": 0.4}, PreferredDifficulty: 0.5, LearningPace: 1},
		hasProfile: true,
	}
	ocr := &fakeAnswerOCR{
		repo:   repo,
		result: answerocrapp.Result{Status: "ok", AnswerLatex: "x+1", Confidence: 0.96, Reason: "清晰"},
	}
	service := newTestService(repo, WithAnswerOCR(ocr))
	service.newID = sequentialIDs("attempt-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:     "exercise-1",
		AnswerImageURL: "/uploads/images/answer.png",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if !ocr.called || ocr.sawTransaction || ocr.imageReference != "/uploads/images/answer.png" || ocr.answerType != "expression" {
		t.Fatalf("OCR call = %#v", ocr)
	}
	if !response.Recorded || response.GradingStatus != "correct" || response.StudentAnswerLatex != "x+1" {
		t.Fatalf("response = %#v", response)
	}
	if len(repo.insertedAttempts) != 1 || repo.insertedAttempts[0].StudentAnswer != "x+1" {
		t.Fatalf("attempts = %#v", repo.insertedAttempts)
	}
}

func TestSubmitAnswerOCRFailuresDoNotMutateLearningState(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "invalid image", err: answerocrapp.ErrInvalidImage, want: ErrBadRequest},
		{name: "unreadable", err: answerocrapp.ErrUnreadable, want: ErrOCRUnreadable},
		{name: "unavailable", err: answerocrapp.ErrUnavailable, want: ErrOCRUnavailable},
		{name: "timeout", err: answerocrapp.ErrTimeout, want: ErrOCRTimeout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeExerciseRepo{
				teacherID:  "teacher-1",
				hasTeacher: true,
				exercises: map[string]Exercise{
					"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "x+1", "answer_type": "expression"}),
				},
			}
			ocr := &fakeAnswerOCR{repo: repo, err: test.err}
			service := newTestService(repo, WithAnswerOCR(ocr))

			_, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
				ExerciseID:     "exercise-1",
				AnswerImageURL: "/uploads/images/answer.png",
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("SubmitAnswer() error = %v, want %v", err, test.want)
			}
			assertNoSubmissionMutation(t, repo)
		})
	}
}

func TestSubmitAnswerTextTakesPriorityOverImageWithoutCallingOCR(t *testing.T) {
	repo := &fakeExerciseRepo{
		session:    LearningSession{ID: "session-1", StudentID: "student-1"},
		hasSession: true,
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "x+1", "answer_type": "expression"}),
		},
		profile:    StudentProfile{MasteryVector: map[string]float64{"algebra": 0.4}, PreferredDifficulty: 0.5, LearningPace: 1},
		hasProfile: true,
	}
	ocr := &fakeAnswerOCR{repo: repo, err: errors.New("must not be called")}
	service := newTestService(repo, WithAnswerOCR(ocr))
	service.newID = sequentialIDs("attempt-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:     "exercise-1",
		AnswerText:     "x+1",
		AnswerImageURL: "https://untrusted.example/ignored.png",
	})
	if err != nil || !response.IsCorrect {
		t.Fatalf("SubmitAnswer() response=%#v error=%v", response, err)
	}
	if ocr.called {
		t.Fatal("OCR was called for a text answer")
	}
}

func TestSubmitAnswerIndeterminateMathResultDoesNotMutateLearningState(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "2*x", "answer_type": "expression"}),
		},
	}
	service := newTestService(repo)

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID: "exercise-1",
		AnswerText: "x+x",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if response.Recorded || response.GradingStatus != "indeterminate" || response.Evaluation.ReasonCode != string(mathsolverapp.ReasonExpressionVerificationNeeded) {
		t.Fatalf("response = %#v", response)
	}
	assertNoSubmissionMutation(t, repo)
}

func TestSubmitAnswerRejectsInvalidOrLowConfidenceDecisionsWithoutLearningWrites(t *testing.T) {
	tests := []struct {
		name       string
		result     AnswerCheckResult
		wantError  error
		wantReason string
	}{
		{
			name: "unknown decision",
			result: AnswerCheckResult{
				Decision: mathsolverapp.Decision("unknown"), Reason: "invalid", Confidence: 1,
			},
			wantError: ErrMathSolverInvalidResult,
		},
		{
			name: "low confidence correct",
			result: AnswerCheckResult{
				Decision: mathsolverapp.DecisionCorrect, Reason: "可能等价", Confidence: 0.69,
			},
			wantReason: "grading_low_confidence",
		},
		{
			name: "high confidence indeterminate",
			result: AnswerCheckResult{
				Decision: mathsolverapp.DecisionIndeterminate, Reason: "无法判定", Confidence: 0.9,
			},
			wantError: ErrMathSolverInvalidResult,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeExerciseRepo{
				teacherID: "teacher-1", hasTeacher: true,
				exercises: map[string]Exercise{
					"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{
						"answer": "2*x", "answer_type": "expression",
					}),
				},
			}
			service, err := NewService(repo, fakeAnswerChecker{result: test.result})
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}

			response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
				ExerciseID: "exercise-1", AnswerText: "x+x",
			})
			if test.wantError != nil {
				if !errors.Is(err, test.wantError) {
					t.Fatalf("SubmitAnswer() error = %v, want %v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatalf("SubmitAnswer() error = %v", err)
				}
				if response.Recorded || response.GradingStatus != string(mathsolverapp.DecisionIndeterminate) || response.Evaluation.ReasonCode != test.wantReason {
					t.Fatalf("response = %#v", response)
				}
			}
			assertNoSubmissionMutation(t, repo)
		})
	}
}

func TestSubmitAnswerChecksAuthorizationBeforeOCR(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID:  "teacher-2",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "x+1", "answer_type": "expression"}),
		},
	}
	ocr := &fakeAnswerOCR{repo: repo, result: answerocrapp.Result{Status: "ok", AnswerLatex: "x+1", Confidence: 1}}
	service := newTestService(repo, WithAnswerOCR(ocr))

	_, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:     "exercise-1",
		AnswerImageURL: "/uploads/images/answer.png",
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("SubmitAnswer() error = %v, want ErrBadRequest", err)
	}
	if ocr.called {
		t.Fatal("OCR was called before authorization")
	}
	assertNoSubmissionMutation(t, repo)
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
	service, err := NewService(repo, fakeAnswerChecker{result: AnswerCheckResult{Decision: mathsolverapp.DecisionIncorrect, Reason: "答案不等价", Confidence: 1}}, WithDiagnostician(diagnostician))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
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
	service, err := NewService(repo, fakeAnswerChecker{result: AnswerCheckResult{Decision: mathsolverapp.DecisionIncorrect, Reason: "答案不等价", Confidence: 1}}, WithDiagnostician(&fakeDiagnostician{err: errors.New("model unavailable")}))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, time.April, 25, 10, 0, 0, 0, time.UTC) }
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

func TestSubmitAnswerKeepsSymbolicTaxonomyForTextErrors(t *testing.T) {
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
	service, err := NewService(repo, fakeAnswerChecker{result: AnswerCheckResult{
		IsCorrect:  false,
		Reason:     "答案符号格式错误",
		Confidence: 1,
	}})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.newID = sequentialIDs("attempt-1", "diagnosis-1", "dkt-1")

	response, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID: "exercise-1",
		AnswerText: "x-1",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() error = %v", err)
	}
	if response.Diagnosis == nil || response.Diagnosis.ErrorType == nil || *response.Diagnosis.ErrorType != "symbolic" || response.Diagnosis.TaxonomyCode != "S-Type" {
		t.Fatalf("diagnosis = %#v", response.Diagnosis)
	}
}

func TestSubmitAnswerRejectsUnsafeImageURL(t *testing.T) {
	cases := []string{
		"https://example.com/answer.png",
		"/uploads/documents/answer.pdf",
		"/uploads/images/../documents/answer.pdf",
		"/uploads/images/answer.png?token=1",
		"/uploads/images/%2e%2e/answer.png",
	}
	for _, imageURL := range cases {
		t.Run(imageURL, func(t *testing.T) {
			repo := &fakeExerciseRepo{}
			service := newTestService(repo)

			_, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
				ExerciseID:     "exercise-1",
				AnswerImageURL: imageURL,
			})
			if !errors.Is(err, ErrBadRequest) {
				t.Fatalf("SubmitAnswer() error = %v, want ErrBadRequest", err)
			}
		})
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
	solver := &fakeSolutionSolver{err: errors.New("must not be called")}
	service := newTestService(repo, WithSolutionSolver(solver))

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if response.Answer != "42" || response.Source != "cached" || len(response.Steps) != 1 {
		t.Fatalf("response = %#v", response)
	}
	if solver.called {
		t.Fatal("solution solver was called despite cached steps")
	}
}

func TestGetSolutionRejectsInvalidMultipleChoiceReferenceBeforeCachedOrGeneratedSteps(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID: "teacher-1", hasTeacher: true, hasSubmitted: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{
				"type": "multiple_choice", "answer_type": "text", "answer": "E",
				"options": []any{"A1", "B1", "C1", "D1"}, "solution_steps": []any{"不得返回的缓存步骤"},
			}),
		},
	}
	solver := &fakeSolutionSolver{err: errors.New("must not be called")}
	service := newTestService(repo, WithSolutionSolver(solver))

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if solver.called || response.Source != "unavailable" || len(response.Steps) != 0 || response.Failure == nil ||
		response.Failure.Code != mathsolverapp.FailureInvalidInput || response.Failure.Stage != "reference_answer" ||
		response.Verification == nil || response.Verification.ReasonCode != "invalid_reference_answer" {
		t.Fatalf("response=%#v solver=%#v", response, solver)
	}
}

func TestGetSolutionGeneratesAndVerifiesMissingSteps(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{
				"answer":         "2*x",
				"answer_type":    "expression",
				"solution_steps": []any{},
			}),
		},
		hasSubmitted: true,
	}
	solver := &fakeSolutionSolver{result: solvedSolution("2*x", "使用乘法法则化简。")}
	service := newTestService(repo, WithSolutionSolver(solver))

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if !solver.called || solver.input.Exercise.ID != "exercise-1" || solver.input.AnswerType != "expression" {
		t.Fatalf("solver = %#v", solver)
	}
	if _, ok := solver.input.Exercise.Meta["answer"]; ok {
		t.Fatalf("solution solver received reference answer: %#v", solver.input.Exercise.Meta)
	}
	if _, ok := solver.input.Exercise.Meta["solution_steps"]; ok {
		t.Fatalf("solution solver received cached solution steps: %#v", solver.input.Exercise.Meta)
	}
	if response.Source != "solver_verified" || len(response.Steps) != 1 || response.Failure != nil || response.Verification == nil {
		t.Fatalf("response = %#v", response)
	}
	if response.Verification.ReasonCode != "solution_steps_verified" || !solver.verifyCalled {
		t.Fatalf("verification = %#v", response.Verification)
	}
	if solver.verificationInput.ReferenceAnswer != "2*x" || solver.verificationInput.CandidateAnswer != "2*x" ||
		len(solver.verificationInput.CandidateSteps) != 1 {
		t.Fatalf("verification input = %#v", solver.verificationInput)
	}
}

func TestGetSolutionUsesGeneralSolverToVerifyEquivalentExpression(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{
				"answer":      "2*x",
				"answer_type": "expression",
			}),
		},
		hasSubmitted: true,
	}
	solutionSolver := &fakeSolutionSolver{result: solvedSolution("x+x", "合并同类项得到 2*x。")}
	mathSolver := &fakeMathSolver{result: AnswerCheckResult{
		Decision:   mathsolverapp.DecisionCorrect,
		Method:     "llm_assisted",
		ReasonCode: "mathematically_equivalent",
		Reason:     "两式代数等价",
		Confidence: 0.94,
		Evidence:   []mathsolverapp.Evidence{{Kind: "identity", Summary: "x+x=2*x"}},
	}}
	service, err := NewService(repo, SolverAnswerChecker{Solver: mathSolver}, WithSolutionSolver(solutionSolver))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if response.Source != "solver_verified" || !mathSolver.called || mathSolver.input.StudentAnswer != "x+x" || mathSolver.input.CorrectAnswer != "2*x" {
		t.Fatalf("response=%#v mathSolver=%#v", response, mathSolver)
	}
}

func TestGetSolutionVerifiesMultipleChoiceCandidateByOptionValue(t *testing.T) {
	options := []any{"函数连续", "函数单调", "函数有界", "函数可导"}
	repo := &fakeExerciseRepo{
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"functions"}, map[string]any{
				"type":        "multiple_choice",
				"answer":      "A",
				"answer_type": "text",
				"options":     options,
			}),
		},
		hasSubmitted: true,
	}
	solver := &fakeSolutionSolver{result: solvedSolution("函数连续", "根据定义选择函数连续。")}
	service := newTestService(repo, WithSolutionSolver(solver))

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if response.Source != "solver_verified" || response.Verification == nil || response.Verification.ReasonCode != "solution_steps_verified" || !solver.verifyCalled {
		t.Fatalf("response = %#v", response)
	}
}

func TestGetSolutionRejectsInvalidStepsEvenWhenFinalAnswerMatches(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID: "teacher-1", hasTeacher: true, hasSubmitted: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"calculus"}, map[string]any{
				"answer": "2*x", "answer_type": "expression",
			}),
		},
	}
	solver := &fakeSolutionSolver{
		result: solvedSolution("2*x", "错误地声称 d(x^2)/dx=3*x。"),
		verificationResult: AnswerCheckResult{
			Decision: mathsolverapp.DecisionIncorrect, Method: "llm_assisted", ReasonCode: "invalid_derivation",
			Reason: "第一步导数公式错误", Confidence: 0.96, Retryable: true,
			Evidence: []mathsolverapp.Evidence{{Kind: "counterexample", Summary: "d(x^2)/dx=2*x，而不是 3*x"}},
		},
	}
	service := newTestService(repo, WithSolutionSolver(solver))

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if !solver.verifyCalled || solver.verificationInput.ReferenceAnswer != "2*x" ||
		len(solver.verificationInput.CandidateSteps) != 1 {
		t.Fatalf("verification call = %#v", solver)
	}
	if response.Source != "unavailable" || len(response.Steps) != 0 || response.Failure == nil ||
		response.Failure.Code != mathsolverapp.FailureVerificationFailed || response.Failure.Message != "第一步导数公式错误" {
		t.Fatalf("response = %#v", response)
	}
}

func TestGetSolutionRejectsExerciseChangesDuringGeneration(t *testing.T) {
	original := newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "2*x", "answer_type": "expression"})
	changed := cloneExercise(original)
	changed.Body = "updated problem body"
	repo := &fakeExerciseRepo{
		teacherID: "teacher-1", hasTeacher: true, hasSubmitted: true,
		exercises: map[string]Exercise{"exercise-1": original}, exerciseOnSecondRead: &changed,
	}
	service := newTestService(repo, WithSolutionSolver(&fakeSolutionSolver{result: solvedSolution("2*x", "step")}))

	_, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if !errors.Is(err, ErrExerciseChanged) {
		t.Fatalf("GetSolution() error = %v, want ErrExerciseChanged", err)
	}
}

func TestGetSolutionDoesNotExposeUnverifiedSteps(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID:  "teacher-1",
		hasTeacher: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"arithmetic"}, map[string]any{
				"answer":      "42",
				"answer_type": "numeric",
			}),
		},
		hasSubmitted: true,
	}
	solver := &fakeSolutionSolver{result: solvedSolution("41", "错误步骤不应返回。")}
	service := newTestService(repo, WithSolutionSolver(solver))

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if response.Source != "unavailable" || len(response.Steps) != 0 || response.Failure == nil || response.Failure.Code != mathsolverapp.FailureVerificationFailed {
		t.Fatalf("response = %#v", response)
	}
	if response.Verification == nil || response.Verification.ReasonCode != string(mathsolverapp.ReasonNumericMismatch) {
		t.Fatalf("verification = %#v", response.Verification)
	}
}

func TestGetSolutionHandlesMissingSolverInputsAndInvalidCandidates(t *testing.T) {
	tests := []struct {
		name     string
		meta     map[string]any
		solver   SolutionSolver
		wantCode mathsolverapp.FailureCode
	}{
		{name: "missing reference", meta: map[string]any{"answer_type": "expression"}, solver: &fakeSolutionSolver{}, wantCode: mathsolverapp.FailureInvalidInput},
		{name: "solver not configured", meta: map[string]any{"answer": "2*x", "answer_type": "expression"}, wantCode: mathsolverapp.FailureSolverUnavailable},
		{
			name: "invalid candidate", meta: map[string]any{"answer": "2*x", "answer_type": "expression"},
			solver:   &fakeSolutionSolver{result: SolutionResult{Status: SolutionStatusSolved, Answer: "2*x", Steps: []string{"step"}, Method: "", ReasonCode: "solved", Reason: "done", Confidence: 0.9}},
			wantCode: mathsolverapp.FailureSolverInvalid,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeExerciseRepo{
				teacherID: "teacher-1", hasTeacher: true, hasSubmitted: true,
				exercises: map[string]Exercise{"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, test.meta)},
			}
			options := []Option{}
			if test.solver != nil {
				options = append(options, WithSolutionSolver(test.solver))
			}
			response, err := newTestService(repo, options...).GetSolution(context.Background(), "student-1", "exercise-1")
			if err != nil {
				t.Fatalf("GetSolution() error = %v", err)
			}
			if response.Source != "unavailable" || response.Failure == nil || response.Failure.Code != test.wantCode || len(response.Steps) != 0 {
				t.Fatalf("response = %#v", response)
			}
		})
	}
}

func TestGetSolutionDoesNotSolveBeforeSubmittedAttempt(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID: "teacher-1", hasTeacher: true,
		exercises: map[string]Exercise{"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "2*x"})},
	}
	solver := &fakeSolutionSolver{result: solvedSolution("2*x", "step")}
	service := newTestService(repo, WithSolutionSolver(solver))

	_, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSolution() error = %v, want ErrNotFound", err)
	}
	if solver.called {
		t.Fatal("solution solver was called before a submitted attempt existed")
	}
}

func TestGetSolutionReturnsExplainableSolverDegradation(t *testing.T) {
	tests := []struct {
		name      string
		result    SolutionResult
		err       error
		wantCode  mathsolverapp.FailureCode
		retryable bool
	}{
		{
			name: "indeterminate",
			result: SolutionResult{
				Status: SolutionStatusIndeterminate, Method: "llm_assisted", ReasonCode: "ambiguous_domain",
				Reason: "题目未给出变量定义域", Confidence: 0.4, Retryable: false,
			},
			wantCode: mathsolverapp.FailureSolverIndeterminate,
		},
		{name: "timeout", err: context.DeadlineExceeded, wantCode: mathsolverapp.FailureSolverTimeout, retryable: true},
		{name: "canceled", err: context.Canceled, wantCode: mathsolverapp.FailureCanceled, retryable: false},
		{name: "invalid response", err: ErrMathSolverInvalidResult, wantCode: mathsolverapp.FailureSolverInvalid, retryable: true},
		{name: "unavailable", err: errors.New("provider unavailable"), wantCode: mathsolverapp.FailureSolverUnavailable, retryable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeExerciseRepo{
				teacherID:  "teacher-1",
				hasTeacher: true,
				exercises: map[string]Exercise{
					"exercise-1": newExercise("exercise-1", "teacher-1", []string{"limit"}, map[string]any{"answer": "1", "answer_type": "numeric"}),
				},
				hasSubmitted: true,
			}
			service := newTestService(repo, WithSolutionSolver(&fakeSolutionSolver{result: test.result, err: test.err}))

			response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
			if err != nil {
				t.Fatalf("GetSolution() error = %v", err)
			}
			if response.Source != "unavailable" || response.Failure == nil || response.Failure.Code != test.wantCode || response.Failure.Retryable != test.retryable {
				t.Fatalf("response = %#v", response)
			}
		})
	}
}

func TestGetSolutionLabelsVerificationFailuresSeparately(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID: "teacher-1", hasTeacher: true, hasSubmitted: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "2*x", "answer_type": "expression"}),
		},
	}
	service, err := NewService(
		repo,
		fakeAnswerChecker{err: context.DeadlineExceeded},
		WithSolutionSolver(&fakeSolutionSolver{result: solvedSolution("x+x", "合并同类项。")}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if response.Failure == nil || response.Failure.Code != mathsolverapp.FailureSolverTimeout ||
		response.Failure.Stage != "solution_verification" || !response.Failure.Retryable {
		t.Fatalf("response = %#v", response)
	}
}

func TestGetSolutionPreservesVerificationFailureExplanation(t *testing.T) {
	repo := &fakeExerciseRepo{
		teacherID: "teacher-1", hasTeacher: true, hasSubmitted: true,
		exercises: map[string]Exercise{
			"exercise-1": newExercise("exercise-1", "teacher-1", []string{"algebra"}, map[string]any{"answer": "2*x", "answer_type": "expression"}),
		},
	}
	service, err := NewService(
		repo,
		fakeAnswerChecker{result: AnswerCheckResult{
			Decision: mathsolverapp.DecisionIndeterminate, Method: "none", ReasonCode: "unsupported",
			Reason: "当前表达式超出自动验证范围", Retryable: false,
			Failure: &mathsolverapp.Failure{Code: mathsolverapp.FailureUnsupportedKind, Stage: "answer_kind", Message: "当前表达式超出自动验证范围", Retryable: false},
		}},
		WithSolutionSolver(&fakeSolutionSolver{result: solvedSolution("x+x", "合并同类项。")}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	response, err := service.GetSolution(context.Background(), "student-1", "exercise-1")
	if err != nil {
		t.Fatalf("GetSolution() error = %v", err)
	}
	if response.Failure == nil || response.Failure.Code != mathsolverapp.FailureVerificationFailed || response.Failure.Retryable ||
		response.Failure.Message != "当前表达式超出自动验证范围" {
		t.Fatalf("response = %#v", response)
	}
}

func TestNormalizeSolutionResultRejectsInvalidBoundaries(t *testing.T) {
	valid := solvedSolution("2*x", "step")
	tests := []struct {
		name   string
		mutate func(*SolutionResult)
	}{
		{name: "missing reason code", mutate: func(result *SolutionResult) { result.ReasonCode = "" }},
		{name: "invalid confidence", mutate: func(result *SolutionResult) { result.Confidence = math.Inf(1) }},
		{name: "too many evidence", mutate: func(result *SolutionResult) { result.Evidence = make([]mathsolverapp.Evidence, 9) }},
		{name: "empty evidence", mutate: func(result *SolutionResult) { result.Evidence = nil }},
		{name: "empty step", mutate: func(result *SolutionResult) { result.Steps = []string{" "} }},
		{name: "unsupported status", mutate: func(result *SolutionResult) { result.Status = "unknown" }},
		{name: "high confidence indeterminate", mutate: func(result *SolutionResult) { result.Status = SolutionStatusIndeterminate }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Steps = append([]string(nil), valid.Steps...)
			candidate.Evidence = append([]mathsolverapp.Evidence(nil), valid.Evidence...)
			test.mutate(&candidate)
			if _, ok := normalizeSolutionResult(candidate); ok {
				t.Fatalf("normalizeSolutionResult(%#v) accepted invalid candidate", candidate)
			}
		})
	}
}

func TestSubmitAnswerRejectsExerciseContextChangesBeforeWrites(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Exercise)
	}{
		{name: "body", change: func(exercise *Exercise) { exercise.Body = "updated body" }},
		{name: "options", change: func(exercise *Exercise) { exercise.Meta["options"] = []any{"40", "41", "42"} }},
		{name: "solution steps", change: func(exercise *Exercise) { exercise.Meta["solution_steps"] = []any{"updated step"} }},
		{name: "tolerance", change: func(exercise *Exercise) { exercise.Meta["absolute_tolerance"] = "0.1" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := newExercise("exercise-1", "teacher-1", []string{"arithmetic"}, map[string]any{
				"type": "short_answer", "answer": "42", "answer_type": "numeric", "absolute_tolerance": "0",
			})
			changed := cloneExercise(original)
			test.change(&changed)
			repo := &fakeExerciseRepo{
				teacherID: "teacher-1", hasTeacher: true,
				exercises: map[string]Exercise{"exercise-1": original}, exerciseOnSecondRead: &changed,
			}
			service := newTestService(repo)

			_, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{ExerciseID: "exercise-1", AnswerText: "42"})
			if !errors.Is(err, ErrExerciseChanged) {
				t.Fatalf("SubmitAnswer() error = %v, want ErrExerciseChanged", err)
			}
			if len(repo.insertedAttempts) != 0 || len(repo.insertedDiagnoses) != 0 || len(repo.upsertedStates) != 0 ||
				repo.createdSession.StudentID != "" || repo.createdProfileUserID != "" || repo.updatedAfterSubmitExerciseID != "" || repo.profileUpdate.TotalExercises != 0 {
				t.Fatalf("exercise change reached learning writes: %#v", repo)
			}
		})
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

func assertNoSubmissionMutation(t *testing.T, repo *fakeExerciseRepo) {
	t.Helper()
	if repo.withTxCalled || len(repo.insertedAttempts) != 0 || len(repo.insertedDiagnoses) != 0 ||
		len(repo.upsertedStates) != 0 || repo.createdSession.StudentID != "" ||
		repo.updatedAfterSubmitExerciseID != "" || len(repo.updatedAttempted) != 0 ||
		repo.createdProfileUserID != "" || repo.profileUpdate.TotalExercises != 0 {
		t.Fatalf("submission mutated learning state: %#v", repo)
	}
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

func cloneExercise(exercise Exercise) Exercise {
	cloned := exercise
	cloned.ConceptIDs = append([]string(nil), exercise.ConceptIDs...)
	cloned.Meta = make(map[string]any, len(exercise.Meta))
	for key, value := range exercise.Meta {
		cloned.Meta[key] = value
	}
	return cloned
}

func solvedSolution(answer string, steps ...string) SolutionResult {
	return SolutionResult{
		Status:     SolutionStatusSolved,
		Answer:     answer,
		Steps:      steps,
		Method:     "llm_assisted",
		ReasonCode: "solved",
		Reason:     "已独立求解",
		Confidence: 0.95,
		Evidence:   []mathsolverapp.Evidence{{Kind: "derivation", Summary: "推导得到最终答案"}},
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

type fakeSolutionSolver struct {
	result             SolutionResult
	input              SolutionInput
	verificationResult AnswerCheckResult
	verificationInput  SolutionVerificationInput
	called             bool
	verifyCalled       bool
	err                error
	verifyErr          error
}

func (s *fakeSolutionSolver) Solve(_ context.Context, input SolutionInput) (SolutionResult, error) {
	s.called = true
	s.input = input
	if s.err != nil {
		return SolutionResult{}, s.err
	}
	return s.result, nil
}

func (s *fakeSolutionSolver) VerifySolution(_ context.Context, input SolutionVerificationInput) (AnswerCheckResult, error) {
	s.verifyCalled = true
	s.verificationInput = input
	if s.verifyErr != nil {
		return AnswerCheckResult{}, s.verifyErr
	}
	if s.verificationResult.Decision != "" || s.verificationResult.Reason != "" || s.verificationResult.Failure != nil {
		return s.verificationResult, nil
	}
	return AnswerCheckResult{
		Decision:   mathsolverapp.DecisionCorrect,
		Method:     "llm_assisted",
		ReasonCode: "solution_steps_verified",
		Reason:     "候选解析的答案和每个步骤均通过独立验证",
		Confidence: 0.95,
		Evidence:   []mathsolverapp.Evidence{{Kind: "derivation", Summary: "逐步验证通过"}},
	}, nil
}

type fakeAnswerOCR struct {
	repo           *fakeExerciseRepo
	result         answerocrapp.Result
	err            error
	called         bool
	sawTransaction bool
	imageReference string
	answerType     string
}

func (o *fakeAnswerOCR) Recognize(_ context.Context, imageReference string, answerType string) (answerocrapp.Result, error) {
	o.called = true
	o.imageReference = imageReference
	o.answerType = answerType
	if o.repo != nil {
		o.sawTransaction = o.repo.withTxCalled
	}
	return o.result, o.err
}

type fakeQuestionGenerator struct {
	question GeneratedQuestion
	input    GenerationInput
	called   bool
	err      error
}

func (g *fakeQuestionGenerator) GenerateQuestion(_ context.Context, input GenerationInput) (GeneratedQuestion, error) {
	g.called = true
	g.input = input
	if g.err != nil {
		return GeneratedQuestion{}, g.err
	}
	return g.question, nil
}

type fakeAnswerChecker struct {
	result AnswerCheckResult
	err    error
}

type recordingAnswerChecker struct {
	result AnswerCheckResult
	called bool
}

func (c *recordingAnswerChecker) CheckAnswer(context.Context, string, string, string) (AnswerCheckResult, error) {
	c.called = true
	return c.result, nil
}

func (c fakeAnswerChecker) CheckAnswer(context.Context, string, string, string) (AnswerCheckResult, error) {
	return c.result, c.err
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
	withTxCalled                 bool
	session                      LearningSession
	hasSession                   bool
	createdSession               LearningSession
	teacherID                    string
	hasTeacher                   bool
	teacherLookupCount           int
	exercises                    map[string]Exercise
	exerciseReadCount            int
	exerciseForUpdateCount       int
	exerciseOnSecondRead         *Exercise
	knowledgeConcept             KnowledgeConcept
	hasKnowledgeConcept          bool
	createdGenerated             GeneratedQuestion
	createdGeneratedStudentID    string
	recentIDs                    []string
	candidateSet                 []Exercise
	lastCandidateFilters         []CandidateFilter
	profile                      StudentProfile
	hasProfile                   bool
	createdProfileUserID         string
	createdProfileAt             time.Time
	createProfileErr             error
	hasSubmitted                 bool
	dktStates                    map[string]DKTState
	interactions                 []LearningInteraction
	updatedCurrent               *string
	updatedAttempted             []string
	updatedAfterSubmitExerciseID string
	insertedAttempts             []AttemptRecord
	insertedDiagnoses            []DiagnosisRecord
	upsertedStates               []DKTState
	profileUpdate                ProfileTrackingUpdate
}

func (r *fakeExerciseRepo) WithTx(ctx context.Context, fn func(context.Context, Repository) error) error {
	r.withTxCalled = true
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

func (r *fakeExerciseRepo) UpdateSessionAfterSubmit(_ context.Context, _ string, exerciseID string, attempted []string) error {
	r.updatedAfterSubmitExerciseID = exerciseID
	r.updatedAttempted = attempted
	return nil
}

func (r *fakeExerciseRepo) GetExercise(_ context.Context, exerciseID string) (Exercise, bool, error) {
	r.exerciseReadCount++
	if r.exerciseReadCount == 2 && r.exerciseOnSecondRead != nil {
		return *r.exerciseOnSecondRead, true, nil
	}
	exercise, ok := r.exercises[exerciseID]
	return exercise, ok, nil
}

func (r *fakeExerciseRepo) GetExerciseForUpdate(ctx context.Context, exerciseID string) (Exercise, bool, error) {
	r.exerciseForUpdateCount++
	return r.GetExercise(ctx, exerciseID)
}

func (r *fakeExerciseRepo) GetKnowledgeConcept(context.Context, string) (KnowledgeConcept, bool, error) {
	return r.knowledgeConcept, r.hasKnowledgeConcept, nil
}

func (r *fakeExerciseRepo) CreateGeneratedExercise(_ context.Context, studentID string, generated GeneratedQuestion, _ time.Time) (Exercise, error) {
	r.createdGeneratedStudentID = studentID
	r.createdGenerated = generated
	meta := map[string]any{
		"type": generated.Type, "answer": generated.Answer, "answer_type": generated.AnswerType,
		"options": generated.Options, "hints": generated.Hints, "solution_steps": generated.SolutionSteps,
		"estimated_time_seconds": generated.EstimatedTimeSeconds, "knowledge_point_names": generated.KnowledgePointNames,
	}
	return Exercise{
		ID: "generated-1", GeneratedByStudentID: studentID, Status: "PUBLISHED", Title: generated.Title,
		Body: generated.Body, Difficulty: generated.Difficulty, ConceptIDs: generated.ConceptIDs, Meta: meta,
	}, nil
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

func (r *fakeExerciseRepo) CreateProfile(_ context.Context, userID string, now time.Time) (StudentProfile, error) {
	r.createdProfileUserID = userID
	r.createdProfileAt = now
	if r.createProfileErr != nil {
		return StudentProfile{}, r.createProfileErr
	}
	r.profile = StudentProfile{
		MasteryVector:       map[string]float64{},
		ErrorTendency:       map[string]float64{},
		PreferredDifficulty: 0.5,
		LearningPace:        1,
	}
	r.hasProfile = true
	return r.profile, nil
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
