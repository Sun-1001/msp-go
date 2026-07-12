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

func TestGenerateExercisePersistsStudentOwnedQuestion(t *testing.T) {
	repo := &fakeExerciseRepo{
		knowledgeConcept:    KnowledgeConcept{ID: "limit", Name: "函数极限", Description: "研究函数趋近行为", Chapter: "第一章"},
		hasKnowledgeConcept: true,
	}
	generator := &fakeQuestionGenerator{question: GeneratedQuestion{
		Title:                "函数极限判断",
		Body:                 "下列关于函数极限的说法正确的是？",
		Type:                 "multiple_choice",
		Answer:               "极限描述函数在某点附近的趋近行为",
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
	if repo.createdGeneratedStudentID != "student-1" || repo.createdGenerated.Difficulty != 0.5 || repo.createdGenerated.AnswerType != "text" {
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

func TestCheckExerciseAnswerFallsBackWithoutStructuredOptions(t *testing.T) {
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
	if !checker.called || !result.IsCorrect {
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
	if result.IsCorrect || result.Reason != "答案与标准答案不一致" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSubmitAnswerRejectsImageOnlyBeforeTransaction(t *testing.T) {
	repo := &fakeExerciseRepo{}
	service := newTestService(repo)

	_, err := service.SubmitAnswer(context.Background(), "student-1", SubmitRequest{
		ExerciseID:     "exercise-1",
		AnswerImageURL: "/uploads/images/answer.png",
	})
	if !errors.Is(err, ErrOCRUnavailable) {
		t.Fatalf("SubmitAnswer() error = %v, want ErrOCRUnavailable", err)
	}
	if repo.withTxCalled || len(repo.insertedAttempts) != 0 || len(repo.insertedDiagnoses) != 0 || len(repo.upsertedStates) != 0 {
		t.Fatalf("image-only answer reached persistence: %#v", repo)
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
	exercise, ok := r.exercises[exerciseID]
	return exercise, ok, nil
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
