package question

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestListQuestionsNormalizesFilter(t *testing.T) {
	repo := &fakeQuestionRepo{questions: []Question{{ID: "question-1"}}, total: 25}
	service := newTestService(repo, time.Date(2026, time.April, 27, 10, 0, 0, 0, time.UTC))

	response, err := service.ListQuestions(context.Background(), "teacher-1", ListFilter{Page: -1, PageSize: 500, SortOrder: "sideways", Tags: []string{" tag ", ""}})
	if err != nil {
		t.Fatalf("ListQuestions() error = %v", err)
	}
	if response.Page != 1 || response.PageSize != 100 || response.Total != 25 {
		t.Fatalf("response = %#v", response)
	}
	if repo.lastOwnerID != "teacher-1" || repo.lastFilter.SortOrder != "desc" || len(repo.lastFilter.Tags) != 1 {
		t.Fatalf("repo call = owner %q filter %#v", repo.lastOwnerID, repo.lastFilter)
	}
}

func TestCreateQuestionAutoMatchesConceptsAndDefaults(t *testing.T) {
	now := time.Date(2026, time.April, 27, 10, 0, 0, 0, time.UTC)
	repo := &fakeQuestionRepo{
		matchedConceptIDs: []string{"concept-1"},
		createQuestion:    Question{ID: "question-1"},
	}
	service := newTestService(repo, now)

	response, err := service.CreateQuestion(context.Background(), "teacher-1", QuestionInput{
		Title:      " 极限与连续 ",
		Body:       " body ",
		Type:       "",
		Difficulty: 2,
		Answer:     "1",
	})
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}
	if response.ID != "question-1" {
		t.Fatalf("response = %#v", response)
	}
	if !repo.matchCalled || repo.lastInput.Type != "short_answer" || repo.lastInput.Difficulty != 1 || repo.lastInput.EstimatedTimeSeconds != 300 {
		t.Fatalf("input = %#v matchCalled=%t", repo.lastInput, repo.matchCalled)
	}
	if len(repo.lastInput.ConceptIDs) != 1 || repo.lastNow != now {
		t.Fatalf("concepts/now = %#v %v", repo.lastInput.ConceptIDs, repo.lastNow)
	}
}

func TestUpdateQuestionMapsMissingToNotFound(t *testing.T) {
	repo := &fakeQuestionRepo{}
	service := newTestService(repo, time.Now())

	_, err := service.UpdateQuestion(context.Background(), "teacher-1", "missing", QuestionUpdate{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateQuestion() error = %v, want ErrNotFound", err)
	}
}

func TestBatchPublishDeduplicatesIDs(t *testing.T) {
	repo := &fakeQuestionRepo{batchCount: 1}
	service := newTestService(repo, time.Now())

	response, err := service.BatchPublish(context.Background(), "teacher-1", []string{"q1", "q1", "", "q2"})
	if err != nil {
		t.Fatalf("BatchPublish() error = %v", err)
	}
	if response.Success != 1 || response.Failed != 1 {
		t.Fatalf("response = %#v", response)
	}
	if len(repo.lastIDs) != 2 {
		t.Fatalf("ids = %#v", repo.lastIDs)
	}
}

func TestBatchImportNormalizesAndMatchesConcepts(t *testing.T) {
	repo := &fakeQuestionRepo{
		matchedConceptIDs: []string{"concept-1"},
		batchResponse:     BatchOperationResponse{Success: 1, Failed: 0, FailedIDs: []string{}, Errors: []string{}},
	}
	service := newTestService(repo, time.Now())

	response, err := service.BatchImport(context.Background(), "teacher-1", []QuestionInput{{Title: "导数", Body: "题目", Answer: "1"}})
	if err != nil {
		t.Fatalf("BatchImport() error = %v", err)
	}
	if response.Success != 1 || len(repo.lastInputs) != 1 || len(repo.lastInputs[0].ConceptIDs) != 1 {
		t.Fatalf("response = %#v inputs = %#v", response, repo.lastInputs)
	}
}

func TestParseQuestionsBuildsShapeCompatibleFallback(t *testing.T) {
	service := newTestService(&fakeQuestionRepo{}, time.Now())

	response, err := service.ParseQuestions(context.Background(), []string{"# 极限题\n求极限"})
	if err != nil {
		t.Fatalf("ParseQuestions() error = %v", err)
	}
	if len(response.Questions) != 1 || response.Questions[0].Title != "极限题" || response.Questions[0].Type != "short_answer" {
		t.Fatalf("response = %#v", response)
	}
}

func TestParseQuestionsUsesConfiguredParser(t *testing.T) {
	parser := &fakeParser{response: AIParseResponse{Questions: []AIParseQuestionItem{{
		Title:         "导数计算",
		Body:          "求 $x^2$ 的导数",
		Type:          "short_answer",
		Difficulty:    1.5,
		Answer:        "2x",
		AnswerType:    "expression",
		Hints:         []string{"使用幂函数求导"},
		SolutionSteps: []string{"应用公式"},
		Tags:          []string{" 导数 "},
	}}}}
	service := newTestService(&fakeQuestionRepo{}, time.Now(), WithParser(parser))

	response, err := service.ParseQuestions(context.Background(), []string{"导数题"})
	if err != nil {
		t.Fatalf("ParseQuestions() error = %v", err)
	}
	if !parser.called || len(parser.input.RawTexts) != 1 || parser.input.Fallback.Questions[0].Title != "导数题" {
		t.Fatalf("parser called=%v input=%#v", parser.called, parser.input)
	}
	if len(response.Questions) != 1 || response.Questions[0].Title != "导数计算" || response.Questions[0].Difficulty != 1 {
		t.Fatalf("response = %#v", response)
	}
	if len(response.Questions[0].Tags) != 1 || response.Questions[0].Tags[0] != "导数" {
		t.Fatalf("tags = %#v", response.Questions[0].Tags)
	}
}

func TestParseQuestionsFallsBackWhenParserFails(t *testing.T) {
	service := newTestService(&fakeQuestionRepo{}, time.Now(), WithParser(&fakeParser{err: errors.New("model unavailable")}))

	response, err := service.ParseQuestions(context.Background(), []string{"# 极限题\n求极限"})
	if err != nil {
		t.Fatalf("ParseQuestions() error = %v", err)
	}
	if len(response.Questions) != 1 || response.Questions[0].Title != "极限题" || response.Questions[0].Answer != "" {
		t.Fatalf("response = %#v", response)
	}
}

func TestGenerateIsomorphicProblemBuildsValidatedIntegralVariant(t *testing.T) {
	service := newTestService(&fakeQuestionRepo{}, time.Now())
	difficulty := 0.75

	response, err := service.GenerateIsomorphicProblem(context.Background(), GenerateRequest{
		Template:   "integral_power_exp",
		Ability:    0.6,
		Difficulty: &difficulty,
		ConceptIDs: []string{"integral"},
		Tags:       []string{"exam"},
	})
	if err != nil {
		t.Fatalf("GenerateIsomorphicProblem() error = %v", err)
	}
	if response.Template != "integral_power_exp" || response.Parameters["n"] < 1 || response.Parameters["a"] < 1 {
		t.Fatalf("response = %#v", response)
	}
	if !response.Validation.HasClosedForm || !response.Validation.InSyllabus || response.Answer == "" {
		t.Fatalf("validation/answer = %#v %q", response.Validation, response.Answer)
	}
	if len(response.Tags) < 3 || response.Tags[0] != "exam" {
		t.Fatalf("tags = %#v", response.Tags)
	}
}

func TestNewServiceRejectsNilRepository(t *testing.T) {
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

type fakeParser struct {
	response AIParseResponse
	input    ParserInput
	called   bool
	err      error
}

func (p *fakeParser) ParseQuestions(_ context.Context, input ParserInput) (AIParseResponse, error) {
	p.called = true
	p.input = input
	if p.err != nil {
		return AIParseResponse{}, p.err
	}
	return p.response, nil
}

type fakeQuestionRepo struct {
	questions         []Question
	total             int
	question          Question
	found             bool
	createQuestion    Question
	updateQuestion    Question
	updateFound       bool
	deleteFound       bool
	groups            []string
	stats             Stats
	batchCount        int
	batchResponse     BatchOperationResponse
	matchedConceptIDs []string
	matchCalled       bool
	lastOwnerID       string
	lastQuestionID    string
	lastFilter        ListFilter
	lastInput         QuestionInput
	lastUpdate        QuestionUpdate
	lastInputs        []QuestionInput
	lastIDs           []string
	lastNow           time.Time
}

func (r *fakeQuestionRepo) MatchConceptIDs(_ context.Context, title string) ([]string, error) {
	r.matchCalled = true
	r.lastInput.Title = title
	return r.matchedConceptIDs, nil
}

func (r *fakeQuestionRepo) ListQuestions(_ context.Context, ownerID string, filter ListFilter) ([]Question, int, error) {
	r.lastOwnerID = ownerID
	r.lastFilter = filter
	return r.questions, r.total, nil
}

func (r *fakeQuestionRepo) GetQuestion(_ context.Context, ownerID string, questionID string) (Question, bool, error) {
	r.lastOwnerID = ownerID
	r.lastQuestionID = questionID
	return r.question, r.found, nil
}

func (r *fakeQuestionRepo) CreateQuestion(_ context.Context, ownerID string, input QuestionInput, now time.Time) (Question, error) {
	r.lastOwnerID = ownerID
	r.lastInput = input
	r.lastNow = now
	return r.createQuestion, nil
}

func (r *fakeQuestionRepo) UpdateQuestion(_ context.Context, ownerID string, questionID string, update QuestionUpdate, now time.Time) (Question, bool, error) {
	r.lastOwnerID = ownerID
	r.lastQuestionID = questionID
	r.lastUpdate = update
	r.lastNow = now
	return r.updateQuestion, r.updateFound, nil
}

func (r *fakeQuestionRepo) DeleteQuestion(_ context.Context, ownerID string, questionID string, now time.Time) (bool, error) {
	r.lastOwnerID = ownerID
	r.lastQuestionID = questionID
	r.lastNow = now
	return r.deleteFound, nil
}

func (r *fakeQuestionRepo) GetGroups(_ context.Context, ownerID string) ([]string, error) {
	r.lastOwnerID = ownerID
	return r.groups, nil
}

func (r *fakeQuestionRepo) GetStats(_ context.Context, ownerID string) (Stats, error) {
	r.lastOwnerID = ownerID
	return r.stats, nil
}

func (r *fakeQuestionRepo) BatchPublish(_ context.Context, ownerID string, ids []string, now time.Time) (int, error) {
	r.lastOwnerID = ownerID
	r.lastIDs = ids
	r.lastNow = now
	return r.batchCount, nil
}

func (r *fakeQuestionRepo) BatchDelete(_ context.Context, ownerID string, ids []string, now time.Time) (int, error) {
	r.lastOwnerID = ownerID
	r.lastIDs = ids
	r.lastNow = now
	return r.batchCount, nil
}

func (r *fakeQuestionRepo) BatchDuplicate(_ context.Context, ownerID string, ids []string, now time.Time) (BatchOperationResponse, error) {
	r.lastOwnerID = ownerID
	r.lastIDs = ids
	r.lastNow = now
	return r.batchResponse, nil
}

func (r *fakeQuestionRepo) BatchImport(_ context.Context, ownerID string, inputs []QuestionInput, now time.Time) (BatchOperationResponse, error) {
	r.lastOwnerID = ownerID
	r.lastInputs = inputs
	r.lastNow = now
	return r.batchResponse, nil
}
