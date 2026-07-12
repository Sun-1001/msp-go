package einoagent

import (
	"context"
	"strings"
	"testing"
	"time"

	adminaiconfigapp "mathstudy/backend-go/internal/application/adminaiconfig"
	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	portraitapp "mathstudy/backend-go/internal/application/portrait"
	questionapp "mathstudy/backend-go/internal/application/question"
	sessionapp "mathstudy/backend-go/internal/application/session"
)

func TestValidateConfigRequiresEnabledModelSettings(t *testing.T) {
	err := validateConfig(Config{Enabled: true, APIKey: "key", Timeout: time.Second, Temperature: 0.3, MaxIterations: 1})
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("validateConfig() error = %v, want missing model", err)
	}
}

func TestValidateConfigAcceptsOpenAICompatibleSettings(t *testing.T) {
	err := validateConfig(Config{
		Enabled:       true,
		BaseURL:       "https://api.example.com/v1",
		APIKey:        "key",
		Model:         "deepseek-chat",
		Timeout:       30 * time.Second,
		Temperature:   0.2,
		MaxTokens:     1000,
		MaxIterations: 4,
	})
	if err != nil {
		t.Fatalf("validateConfig() error = %v", err)
	}
}

func TestConfigurableTutorAgentUsesPersistedRuntimeConfigBeforeFallback(t *testing.T) {
	topP := 0.9
	provider := &fakeRuntimeConfigProvider{
		runtime: adminaiconfigapp.RuntimeConfig{
			BaseURL:       "https://api.example.com",
			APIKey:        "persisted-key",
			Model:         "persisted-model",
			Temperature:   0.2,
			MaxTokens:     100,
			TopP:          &topP,
			Timeout:       time.Second,
			MaxIterations: 2,
		},
		ok: true,
	}
	agent := NewConfigurableTutorAgent(provider, Config{Enabled: false})
	var captured Config
	agent.newAgent = func(_ context.Context, cfg Config) (sessionapp.ChatAgent, error) {
		captured = cfg
		return &fakeChatAgent{output: sessionapp.ChatAgentOutput{Agent: "tutor", Content: "ok"}}, nil
	}
	output, err := agent.Generate(context.Background(), sessionapp.ChatAgentInput{Message: "ping"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !provider.called || provider.agentType != "tutor" {
		t.Fatalf("provider called=%v agentType=%q", provider.called, provider.agentType)
	}
	if output.Content != "ok" || captured.Model != "persisted-model" || captured.APIKey != "persisted-key" || captured.TopP == nil || *captured.TopP != topP {
		t.Fatalf("output=%#v captured=%#v", output, captured)
	}
}

func TestConfigurablePortraitGeneratorUsesPortraitRuntimeConfig(t *testing.T) {
	provider := &fakeRuntimeConfigProvider{
		runtime: adminaiconfigapp.RuntimeConfig{
			BaseURL:       "https://api.example.com",
			APIKey:        "portrait-key",
			Model:         "portrait-model",
			Temperature:   0.4,
			MaxTokens:     800,
			Timeout:       time.Second,
			MaxIterations: 3,
		},
		ok: true,
	}
	generator := NewConfigurablePortraitGenerator(provider, Config{Enabled: false})
	var captured Config
	generator.newGenerator = func(_ context.Context, cfg Config) (portraitapp.Generator, error) {
		captured = cfg
		return fakePortraitGenerator{content: "# 画像"}, nil
	}
	content, err := generator.GeneratePortrait(context.Background(), portraitapp.GeneratorInput{
		Profile: portraitapp.Profile{StudentID: "student-1"},
	})
	if err != nil {
		t.Fatalf("GeneratePortrait() error = %v", err)
	}
	if content != "# 画像" || !provider.called || provider.agentType != "portrait" {
		t.Fatalf("content=%q called=%v agentType=%q", content, provider.called, provider.agentType)
	}
	if captured.Model != "portrait-model" || captured.APIKey != "portrait-key" || captured.MaxIterations != 3 {
		t.Fatalf("captured config = %#v", captured)
	}
}

func TestConfigurableDiagnosticianUsesDiagnosticianRuntimeConfig(t *testing.T) {
	provider := &fakeRuntimeConfigProvider{
		runtime: adminaiconfigapp.RuntimeConfig{
			BaseURL:       "https://api.example.com",
			APIKey:        "diagnosis-key",
			Model:         "diagnosis-model",
			Temperature:   0.1,
			MaxTokens:     600,
			Timeout:       time.Second,
			MaxIterations: 2,
		},
		ok: true,
	}
	diagnostician := NewConfigurableDiagnostician(provider, Config{Enabled: false})
	var captured Config
	diagnostician.newDiagnostician = func(_ context.Context, cfg Config) (exerciseapp.Diagnostician, error) {
		captured = cfg
		errorType := "procedural"
		return fakeExerciseDiagnostician{diagnosis: exerciseapp.DiagnosisDetail{
			ErrorType:        &errorType,
			ErrorSubtype:     "answer_mismatch",
			TaxonomyCode:     "P-Type",
			ErrorDescription: "步骤执行有误",
			Severity:         "medium",
			Suggestion:       "复查计算过程。",
		}}, nil
	}
	diagnosis, err := diagnostician.Diagnose(context.Background(), exerciseapp.DiagnosisInput{})
	if err != nil {
		t.Fatalf("Diagnose() error = %v", err)
	}
	if !provider.called || provider.agentType != "diagnostician" || diagnosis.TaxonomyCode != "P-Type" {
		t.Fatalf("called=%v agentType=%q diagnosis=%#v", provider.called, provider.agentType, diagnosis)
	}
	if captured.Model != "diagnosis-model" || captured.APIKey != "diagnosis-key" || captured.MaxIterations != 2 {
		t.Fatalf("captured config = %#v", captured)
	}
}

func TestConfigurableMathSolverUsesMathSolverRuntimeConfig(t *testing.T) {
	provider := &fakeRuntimeConfigProvider{
		runtime: adminaiconfigapp.RuntimeConfig{
			BaseURL:       "https://api.example.com",
			APIKey:        "math-key",
			Model:         "math-model",
			Temperature:   0,
			MaxTokens:     200,
			Timeout:       time.Second,
			MaxIterations: 1,
		},
		ok: true,
	}
	solver := NewConfigurableMathSolver(provider, Config{Enabled: false})
	var captured Config
	solver.newSolver = func(_ context.Context, cfg Config) (exerciseapp.MathSolver, error) {
		captured = cfg
		return fakeEinoMathSolver{result: exerciseapp.AnswerCheckResult{IsCorrect: true, Reason: "等价", Confidence: 0.95}}, nil
	}
	result, err := solver.CheckAnswer(context.Background(), exerciseapp.AnswerCheckInput{})
	if err != nil {
		t.Fatalf("CheckAnswer() error = %v", err)
	}
	if !provider.called || provider.agentType != "math_solver" || !result.IsCorrect {
		t.Fatalf("called=%v agentType=%q result=%#v", provider.called, provider.agentType, result)
	}
	if captured.Model != "math-model" || captured.APIKey != "math-key" || captured.MaxIterations != 1 {
		t.Fatalf("captured config = %#v", captured)
	}
}

func TestConfigurableQuestionParserUsesQuestionParserRuntimeConfig(t *testing.T) {
	provider := &fakeRuntimeConfigProvider{
		runtime: adminaiconfigapp.RuntimeConfig{
			BaseURL:       "https://api.example.com",
			APIKey:        "parser-key",
			Model:         "parser-model",
			Temperature:   0.2,
			MaxTokens:     900,
			Timeout:       time.Second,
			MaxIterations: 2,
		},
		ok: true,
	}
	parser := NewConfigurableQuestionParser(provider, Config{Enabled: false})
	var captured Config
	parser.newParser = func(_ context.Context, cfg Config) (questionapp.Parser, error) {
		captured = cfg
		return fakeQuestionParser{response: questionapp.AIParseResponse{Questions: []questionapp.AIParseQuestionItem{{Title: "题目", Body: "body"}}}}, nil
	}
	response, err := parser.ParseQuestions(context.Background(), questionapp.ParserInput{})
	if err != nil {
		t.Fatalf("ParseQuestions() error = %v", err)
	}
	if !provider.called || provider.agentType != "question_parser" || len(response.Questions) != 1 {
		t.Fatalf("called=%v agentType=%q response=%#v", provider.called, provider.agentType, response)
	}
	if captured.Model != "parser-model" || captured.APIKey != "parser-key" || captured.MaxIterations != 2 {
		t.Fatalf("captured config = %#v", captured)
	}
}

func TestConfigurableQuestionGeneratorUsesQuestionGeneratorRuntimeConfig(t *testing.T) {
	provider := &fakeRuntimeConfigProvider{
		runtime: adminaiconfigapp.RuntimeConfig{
			BaseURL:       "https://api.example.com",
			APIKey:        "generator-key",
			Model:         "generator-model",
			Temperature:   0.35,
			MaxTokens:     1200,
			Timeout:       time.Second,
			MaxIterations: 3,
		},
		ok: true,
	}
	generator := NewConfigurableQuestionGenerator(provider, Config{Enabled: false})
	var captured Config
	generator.newGenerator = func(_ context.Context, cfg Config) (exerciseapp.QuestionGenerator, error) {
		captured = cfg
		return fakeExerciseQuestionGenerator{question: exerciseapp.GeneratedQuestion{Title: "AI 题目"}}, nil
	}
	question, err := generator.GenerateQuestion(context.Background(), exerciseapp.GenerationInput{
		Concept:    exerciseapp.KnowledgeConcept{ID: "concept-1", Name: "导数"},
		Difficulty: 0.6,
	})
	if err != nil {
		t.Fatalf("GenerateQuestion() error = %v", err)
	}
	if question.Title != "AI 题目" || !provider.called || provider.agentType != "question_generator" {
		t.Fatalf("question=%#v called=%v agentType=%q", question, provider.called, provider.agentType)
	}
	if captured.Model != "generator-model" || captured.APIKey != "generator-key" || captured.MaxIterations != 3 {
		t.Fatalf("captured config = %#v", captured)
	}
}

func TestPortraitGeneratorBuildsPromptFromProfile(t *testing.T) {
	agent := &fakeChatAgent{output: sessionapp.ChatAgentOutput{Agent: "portrait", Content: "  # 学生画像\n  "}}
	generator := portraitGenerator{agent: agent}
	content, err := generator.GeneratePortrait(context.Background(), portraitapp.GeneratorInput{
		Profile: portraitapp.Profile{
			StudentID:             "student-1",
			MasteryVector:         map[string]float64{"导数": 0.8},
			ErrorTendency:         map[string]float64{"conceptual": 2},
			TotalExercises:        8,
			CorrectCount:          6,
			TotalStudyTimeMinutes: 120,
			RecentConcepts:        []string{"导数"},
		},
		FallbackContent: "模板报告",
	})
	if err != nil {
		t.Fatalf("GeneratePortrait() error = %v", err)
	}
	if content != "# 学生画像" {
		t.Fatalf("content = %q", content)
	}
	prompt := agent.lastInput.Message
	for _, want := range []string{"student-1", "总练习次数: 8", "导数", "模板报告"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}

func TestExerciseDiagnosticianParsesStructuredJSON(t *testing.T) {
	agent := &fakeChatAgent{output: sessionapp.ChatAgentOutput{Agent: "diagnostician", Content: `{
		"error_type":"symbolic",
		"error_subtype":"notation_error",
		"taxonomy_code":"S-Type",
		"error_description":"符号书写不规范",
		"error_step_index":1,
		"severity":"medium",
		"suggestion":"检查上下标与等号链。",
		"related_concepts":["导数"]
	}`}}
	diagnostician := exerciseDiagnostician{agent: agent}
	fallbackType := "procedural"
	diagnosis, err := diagnostician.Diagnose(context.Background(), exerciseapp.DiagnosisInput{
		Exercise:      exerciseapp.Exercise{ID: "exercise-1", Title: "导数题", Body: "求导", Difficulty: 0.4, ConceptIDs: []string{"导数"}},
		StudentAnswer: "x^2",
		CorrectAnswer: "2x",
		Check:         exerciseapp.AnswerCheckResult{Reason: "答案与标准答案不一致", Confidence: 0.3},
		Fallback: exerciseapp.DiagnosisDetail{
			ErrorType:        &fallbackType,
			ErrorSubtype:     "answer_mismatch",
			TaxonomyCode:     "P-Type",
			ErrorDescription: "答案不一致",
			Severity:         "medium",
			Suggestion:       "复算。",
		},
	})
	if err != nil {
		t.Fatalf("Diagnose() error = %v", err)
	}
	if diagnosis.ErrorType == nil || *diagnosis.ErrorType != "symbolic" || diagnosis.TaxonomyCode != "S-Type" || diagnosis.ErrorStepIndex == nil || *diagnosis.ErrorStepIndex != 1 {
		t.Fatalf("diagnosis = %#v", diagnosis)
	}
	prompt := agent.lastInput.Message
	for _, want := range []string{"导数题", "学生答案: x^2", "标准答案: 2x", "本地兜底诊断"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}

func TestExerciseMathSolverParsesStructuredJSON(t *testing.T) {
	agent := &fakeChatAgent{output: sessionapp.ChatAgentOutput{Agent: "math_solver", Content: `{"is_correct":true,"reason":"两式代数等价","confidence":0.91}`}}
	solver := exerciseMathSolver{agent: agent}
	result, err := solver.CheckAnswer(context.Background(), exerciseapp.AnswerCheckInput{
		StudentAnswer: "x+x",
		CorrectAnswer: "2x",
		AnswerType:    "expression",
		Fallback:      exerciseapp.AnswerCheckResult{IsCorrect: false, Reason: "答案与标准答案不一致", Confidence: 0.3},
	})
	if err != nil {
		t.Fatalf("CheckAnswer() error = %v", err)
	}
	if !result.IsCorrect || result.Confidence != 0.91 || result.Reason != "两式代数等价" {
		t.Fatalf("result = %#v", result)
	}
	prompt := agent.lastInput.Message
	for _, want := range []string{"学生答案: x+x", "标准答案: 2x", "本地兜底判定"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}

func TestQuestionParserParsesStructuredJSON(t *testing.T) {
	agent := &fakeChatAgent{output: sessionapp.ChatAgentOutput{Agent: "question_parser", Content: `{
		"questions":[{
			"title":"导数计算",
			"body":"求 x^2 的导数",
			"type":"short_answer",
			"difficulty":0.4,
			"answer":"2x",
			"answer_type":"expression",
			"options":[],
			"hints":["幂函数求导"],
			"solution_steps":["套用公式"],
			"tags":["导数"]
		}]
	}`}}
	parser := questionParser{agent: agent}
	response, err := parser.ParseQuestions(context.Background(), questionapp.ParserInput{
		RawTexts: []string{"导数题"},
		Fallback: questionapp.AIParseResponse{Questions: []questionapp.AIParseQuestionItem{{Title: "导数题", Body: "导数题"}}},
	})
	if err != nil {
		t.Fatalf("ParseQuestions() error = %v", err)
	}
	if len(response.Questions) != 1 || response.Questions[0].Title != "导数计算" || response.Questions[0].Answer != "2x" {
		t.Fatalf("response = %#v", response)
	}
	prompt := agent.lastInput.Message
	for _, want := range []string{"原始文本 1", "导数题", "本地兜底解析"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}

func TestExerciseQuestionGeneratorBuildsPromptAndUsesTrustedContext(t *testing.T) {
	agent := &fakeChatAgent{output: sessionapp.ChatAgentOutput{Agent: "question_generator", Content: `{
		"title":"隐函数求导",
		"body":"设 x^2+y^2=1，求 dy/dx。",
		"type":"multiple_choice",
		"difficulty":0.3,
		"answer":"-x/y",
		"answer_type":"text",
		"options":["x/y","-x/y","-y/x","y/x"],
		"hints":["等式两边同时对 x 求导。"],
		"solution_steps":["得到 2x+2y y'=0。","解得 y'=-x/y。"],
		"estimated_time_seconds":180,
		"concept_ids":["hallucinated"],
		"knowledge_point_names":["无关知识点"]
	}`}}
	generator := exerciseQuestionGenerator{agent: agent}
	question, err := generator.GenerateQuestion(context.Background(), exerciseapp.GenerationInput{
		Concept: exerciseapp.KnowledgeConcept{
			ID:          "implicit-differentiation",
			Name:        "隐函数求导",
			Description: "通过方程两边求导计算导数",
			Chapter:     "导数与微分",
		},
		Difficulty: 0.7,
	})
	if err != nil {
		t.Fatalf("GenerateQuestion() error = %v", err)
	}
	if question.Difficulty != 0.7 || strings.Join(question.ConceptIDs, ",") != "implicit-differentiation" || strings.Join(question.KnowledgePointNames, ",") != "隐函数求导" {
		t.Fatalf("question trusted fields = %#v", question)
	}
	prompt := agent.lastInput.Message
	for _, want := range []string{"隐函数求导", "导数与微分", `"difficulty":0.7`, "multiple_choice", "30 到 3600"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}

func TestParseGeneratedQuestionJSONAcceptsValidMultipleChoiceQuestion(t *testing.T) {
	question, err := parseGeneratedQuestionJSON(`{
		"title":"  定积分计算  ",
		"body":"计算积分。",
		"type":"multiple_choice",
		"difficulty":0.5,
		"answer":" 2 ",
		"answer_type":"text",
		"options":[" 1 "," 2 "," 3 "," 4 "],
		"hints":[" 使用牛顿-莱布尼茨公式 "],
		"solution_steps":[" 代入上下限 "],
		"estimated_time_seconds":240,
		"concept_ids":["integral"],
		"knowledge_point_names":["定积分"]
	}`)
	if err != nil {
		t.Fatalf("parseGeneratedQuestionJSON() error = %v", err)
	}
	if question.Title != "定积分计算" || question.Answer != "2" || len(question.Options) != 4 || question.Options[1] != "2" || question.EstimatedTimeSeconds != 240 {
		t.Fatalf("question = %#v", question)
	}
}

func TestParseGeneratedQuestionJSONRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		options string
		answer  string
		want    string
	}{
		{name: "duplicate after trim", options: `["1"," 1 ","2","3"]`, answer: "1", want: "four unique"},
		{name: "extra empty option", options: `["1","2","3","4"," "]`, answer: "1", want: "four unique"},
		{name: "answer not present", options: `["1","2","3","4"]`, answer: "5", want: "match one option"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := `{"title":"题目","body":"题干","type":"multiple_choice","difficulty":0.5,"answer":"` + tt.answer + `","answer_type":"text","options":` + tt.options + `,"hints":["提示"],"solution_steps":["解析"],"estimated_time_seconds":120}`
			_, err := parseGeneratedQuestionJSON(content)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseGeneratedQuestionJSON() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseQuestionParseJSONRejectsMissingBody(t *testing.T) {
	_, err := parseQuestionParseJSON(`{"questions":[{"title":"题目","body":""}]}`)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("parseQuestionParseJSON() error = %v, want required fields", err)
	}
}

func TestParseAnswerCheckJSONRejectsInvalidConfidence(t *testing.T) {
	_, err := parseAnswerCheckJSON(`{"is_correct":true,"reason":"ok","confidence":1.2}`)
	if err == nil || !strings.Contains(err.Error(), "confidence") {
		t.Fatalf("parseAnswerCheckJSON() error = %v, want confidence error", err)
	}
}

func TestParseDiagnosisJSONRejectsMismatchedTaxonomy(t *testing.T) {
	_, err := parseDiagnosisJSON(`{
		"error_type":"conceptual",
		"error_subtype":"definition_confusion",
		"taxonomy_code":"P-Type",
		"error_description":"概念混淆",
		"severity":"high",
		"suggestion":"复习定义。",
		"related_concepts":[]
	}`)
	if err == nil || !strings.Contains(err.Error(), "taxonomy_code") {
		t.Fatalf("parseDiagnosisJSON() error = %v, want taxonomy mismatch", err)
	}
}

func TestToMessagesKeepsHistoryAndAttachmentContext(t *testing.T) {
	messages := toMessages(sessionapp.ChatAgentInput{
		Message:     "讲一下洛必达法则",
		Attachments: []string{"/uploads/images/a.png"},
		History: []sessionapp.Message{
			{Role: "assistant", Content: "先看极限定义"},
			{Role: "user", Content: "我不理解"},
			{Role: "system", Content: "ignored"},
		},
	})
	if len(messages) != 3 {
		t.Fatalf("messages = %#v", messages)
	}
	if messages[0].Content != "先看极限定义" || messages[1].Content != "我不理解" {
		t.Fatalf("history messages = %#v", messages)
	}
	if !strings.Contains(messages[2].Content, "洛必达法则") || !strings.Contains(messages[2].Content, "/uploads/images/a.png") {
		t.Fatalf("user message = %q", messages[2].Content)
	}
}

type fakeRuntimeConfigProvider struct {
	runtime   adminaiconfigapp.RuntimeConfig
	ok        bool
	called    bool
	agentType string
	err       error
}

func (p *fakeRuntimeConfigProvider) RuntimeConfig(_ context.Context, agentType string) (adminaiconfigapp.RuntimeConfig, bool, error) {
	p.called = true
	p.agentType = agentType
	if p.err != nil {
		return adminaiconfigapp.RuntimeConfig{}, false, p.err
	}
	return p.runtime, p.ok, nil
}

type fakeChatAgent struct {
	output    sessionapp.ChatAgentOutput
	lastInput sessionapp.ChatAgentInput
	err       error
}

func (a *fakeChatAgent) Generate(_ context.Context, input sessionapp.ChatAgentInput) (sessionapp.ChatAgentOutput, error) {
	a.lastInput = input
	if a.err != nil {
		return sessionapp.ChatAgentOutput{}, a.err
	}
	return a.output, nil
}

type fakePortraitGenerator struct {
	content string
	err     error
}

type fakeExerciseDiagnostician struct {
	diagnosis exerciseapp.DiagnosisDetail
	err       error
}

type fakeEinoMathSolver struct {
	result exerciseapp.AnswerCheckResult
	err    error
}

type fakeQuestionParser struct {
	response questionapp.AIParseResponse
	err      error
}

type fakeExerciseQuestionGenerator struct {
	question exerciseapp.GeneratedQuestion
	err      error
}

func (g fakeExerciseQuestionGenerator) GenerateQuestion(context.Context, exerciseapp.GenerationInput) (exerciseapp.GeneratedQuestion, error) {
	if g.err != nil {
		return exerciseapp.GeneratedQuestion{}, g.err
	}
	return g.question, nil
}

func (p fakeQuestionParser) ParseQuestions(context.Context, questionapp.ParserInput) (questionapp.AIParseResponse, error) {
	if p.err != nil {
		return questionapp.AIParseResponse{}, p.err
	}
	return p.response, nil
}

func (s fakeEinoMathSolver) CheckAnswer(context.Context, exerciseapp.AnswerCheckInput) (exerciseapp.AnswerCheckResult, error) {
	if s.err != nil {
		return exerciseapp.AnswerCheckResult{}, s.err
	}
	return s.result, nil
}

func (d fakeExerciseDiagnostician) Diagnose(context.Context, exerciseapp.DiagnosisInput) (exerciseapp.DiagnosisDetail, error) {
	if d.err != nil {
		return exerciseapp.DiagnosisDetail{}, d.err
	}
	return d.diagnosis, nil
}

func (g fakePortraitGenerator) GeneratePortrait(context.Context, portraitapp.GeneratorInput) (string, error) {
	if g.err != nil {
		return "", g.err
	}
	return g.content, nil
}
