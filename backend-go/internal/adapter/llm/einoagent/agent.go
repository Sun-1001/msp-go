package einoagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	adminaiconfigapp "mathstudy/backend-go/internal/application/adminaiconfig"
	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	portraitapp "mathstudy/backend-go/internal/application/portrait"
	questionapp "mathstudy/backend-go/internal/application/question"
	sessionapp "mathstudy/backend-go/internal/application/session"
)

const tutorInstruction = `你是高等数学智能学习平台的导师智能体。
目标：用中文给学生提供清晰、分步骤、可执行的辅导。
约束：
- 优先解释思路，不直接跳到结论。
- 对公式使用 LaTeX。
- 如果题目或上下文不足，先说明缺失信息并给出下一步建议。
- 不编造学生画像、课程数据或题库中不存在的信息。`

const portraitInstruction = `你是高等数学学习平台的学生画像智能体。
目标：基于平台提供的学习统计生成准确、克制、可行动的中文画像报告。
约束：
- 只使用输入中的学习数据和模板基线，不编造不存在的课程、考试或个人信息。
- 输出 Markdown，包含学习概况、优势、风险点和下一步建议。
- 建议要具体、温和，并能被学生或教师直接执行。
- 数据不足时明确说明置信度有限，并给出最小可行的下一步。`

const diagnosticianInstruction = `你是高等数学学习平台的错因诊断智能体。
目标：基于题目、标准答案、学生答案、步骤和本地判题结果，输出结构化错因诊断。
约束：
- 只输出 JSON，不要输出 Markdown 或解释性前后缀。
- error_type 只能是 conceptual、procedural、logical、symbolic 之一。
- severity 只能是 low、medium、high 之一。
- taxonomy_code 必须与 error_type 对应：conceptual=C-Type，procedural=P-Type，logical=L-Type，symbolic=S-Type。
- 不编造题目之外的学习记录或学生个人信息。
- 如果信息不足，使用 procedural/answer_mismatch 并给出可执行复查建议。`

const mathSolverInstruction = `你是高等数学学习平台的答案等价判定智能体。
目标：比较学生答案与标准答案在数学意义上是否等价。
约束：
- 只输出 JSON，不要输出 Markdown 或解释性前后缀。
- JSON 字段必须包含 is_correct、reason、confidence。
- confidence 必须在 0 到 1 之间。
- 不要因为表达形式不同就判错；允许代数等价、常数项等价和常见 LaTeX/文本差异。
- 信息不足或无法可靠判断时，is_correct=false，confidence 不超过 0.5，并说明需要人工复核。`

const questionParserInstruction = `你是高等数学学习平台的题目解析智能体。
目标：从教师粘贴的原始文本中抽取题目候选。
约束：
- 只输出 JSON，不要输出 Markdown 或解释性前后缀。
- JSON 顶层必须是 {"questions":[...]}。
- 每个题目包含 title、body、type、difficulty、answer、answer_type、options、hints、solution_steps、tags。
- type 只能是 short_answer、multiple_choice、proof；answer_type 只能是 expression、numeric、text。
- difficulty 必须在 0 到 1 之间。
- 信息缺失时使用空字符串或空数组，不要编造标准答案。`

// Config stores Eino runtime settings for the tutor agent.
type Config struct {
	Enabled       bool
	BaseURL       string
	APIKey        string
	Model         string
	Timeout       time.Duration
	Temperature   float64
	MaxTokens     int
	TopP          *float64
	MaxIterations int
}

// Agent adapts Eino ADK to the session ChatAgent interface.
type Agent struct {
	name   string
	runner *adk.Runner
}

// RuntimeConfigProvider loads persisted agent runtime configuration.
type RuntimeConfigProvider interface {
	RuntimeConfig(context.Context, string) (adminaiconfigapp.RuntimeConfig, bool, error)
}

// ConfigurableAgent resolves the Tutor Agent runtime from persisted admin AI config,
// falling back to EINO_* environment settings when no persisted config exists.
type ConfigurableAgent struct {
	provider RuntimeConfigProvider
	fallback Config
	newAgent func(context.Context, Config) (sessionapp.ChatAgent, error)
}

// ConfigurablePortraitGenerator resolves the Portrait Agent runtime per request.
type ConfigurablePortraitGenerator struct {
	provider     RuntimeConfigProvider
	fallback     Config
	newGenerator func(context.Context, Config) (portraitapp.Generator, error)
}

// ConfigurableDiagnostician resolves the Diagnostician Agent runtime per request.
type ConfigurableDiagnostician struct {
	provider         RuntimeConfigProvider
	fallback         Config
	newDiagnostician func(context.Context, Config) (exerciseapp.Diagnostician, error)
}

// ConfigurableMathSolver resolves the Math Solver Agent runtime per request.
type ConfigurableMathSolver struct {
	provider  RuntimeConfigProvider
	fallback  Config
	newSolver func(context.Context, Config) (exerciseapp.MathSolver, error)
}

// ConfigurableQuestionParser resolves the Question Parser Agent runtime per request.
type ConfigurableQuestionParser struct {
	provider  RuntimeConfigProvider
	fallback  Config
	newParser func(context.Context, Config) (questionapp.Parser, error)
}

type chatAgentSpec struct {
	name        string
	description string
	instruction string
}

// NewConfigurableTutorAgent creates a Tutor Agent that reads runtime config per request.
func NewConfigurableTutorAgent(provider RuntimeConfigProvider, fallback Config) *ConfigurableAgent {
	return &ConfigurableAgent{
		provider: provider,
		fallback: fallback,
		newAgent: func(ctx context.Context, cfg Config) (sessionapp.ChatAgent, error) {
			return NewTutorAgent(ctx, cfg)
		},
	}
}

// NewConfigurablePortraitGenerator creates a portrait generator backed by admin AI config.
func NewConfigurablePortraitGenerator(provider RuntimeConfigProvider, fallback Config) *ConfigurablePortraitGenerator {
	return &ConfigurablePortraitGenerator{
		provider: provider,
		fallback: fallback,
		newGenerator: func(ctx context.Context, cfg Config) (portraitapp.Generator, error) {
			return NewPortraitGenerator(ctx, cfg)
		},
	}
}

// NewConfigurableDiagnostician creates a diagnosis generator backed by admin AI config.
func NewConfigurableDiagnostician(provider RuntimeConfigProvider, fallback Config) *ConfigurableDiagnostician {
	return &ConfigurableDiagnostician{
		provider: provider,
		fallback: fallback,
		newDiagnostician: func(ctx context.Context, cfg Config) (exerciseapp.Diagnostician, error) {
			return NewDiagnostician(ctx, cfg)
		},
	}
}

// NewConfigurableMathSolver creates an answer checker backed by admin AI config.
func NewConfigurableMathSolver(provider RuntimeConfigProvider, fallback Config) *ConfigurableMathSolver {
	return &ConfigurableMathSolver{
		provider: provider,
		fallback: fallback,
		newSolver: func(ctx context.Context, cfg Config) (exerciseapp.MathSolver, error) {
			return NewMathSolver(ctx, cfg)
		},
	}
}

// NewConfigurableQuestionParser creates a parser backed by admin AI config.
func NewConfigurableQuestionParser(provider RuntimeConfigProvider, fallback Config) *ConfigurableQuestionParser {
	return &ConfigurableQuestionParser{
		provider: provider,
		fallback: fallback,
		newParser: func(ctx context.Context, cfg Config) (questionapp.Parser, error) {
			return NewQuestionParser(ctx, cfg)
		},
	}
}

// NewTutorAgent creates an Eino ChatModelAgent backed by an OpenAI-compatible model.
func NewTutorAgent(ctx context.Context, cfg Config) (*Agent, error) {
	return newChatModelAgent(ctx, cfg, chatAgentSpec{
		name:        "tutor",
		description: "高等数学学习辅导智能体，负责讲解概念、分析解题思路和给出练习建议。",
		instruction: tutorInstruction,
	})
}

// NewPortraitAgent creates an Eino ChatModelAgent for student portrait generation.
func NewPortraitAgent(ctx context.Context, cfg Config) (*Agent, error) {
	return newChatModelAgent(ctx, cfg, chatAgentSpec{
		name:        "portrait",
		description: "高等数学学生画像智能体，负责基于学习统计生成画像报告和下一步建议。",
		instruction: portraitInstruction,
	})
}

// NewDiagnosticianAgent creates an Eino ChatModelAgent for structured exercise diagnosis.
func NewDiagnosticianAgent(ctx context.Context, cfg Config) (*Agent, error) {
	return newChatModelAgent(ctx, cfg, chatAgentSpec{
		name:        "diagnostician",
		description: "高等数学错因诊断智能体，负责输出结构化 C/P/L/S-Type 错因和复习建议。",
		instruction: diagnosticianInstruction,
	})
}

// NewMathSolverAgent creates an Eino ChatModelAgent for answer equivalence checks.
func NewMathSolverAgent(ctx context.Context, cfg Config) (*Agent, error) {
	return newChatModelAgent(ctx, cfg, chatAgentSpec{
		name:        "math_solver",
		description: "高等数学答案等价判定智能体，负责结构化比较学生答案与标准答案。",
		instruction: mathSolverInstruction,
	})
}

// NewQuestionParserAgent creates an Eino ChatModelAgent for question parsing.
func NewQuestionParserAgent(ctx context.Context, cfg Config) (*Agent, error) {
	return newChatModelAgent(ctx, cfg, chatAgentSpec{
		name:        "question_parser",
		description: "高等数学题目解析智能体，负责把原始文本抽取为题库候选结构。",
		instruction: questionParserInstruction,
	})
}

// NewPortraitGenerator adapts a portrait Eino agent to the portrait application interface.
func NewPortraitGenerator(ctx context.Context, cfg Config) (portraitapp.Generator, error) {
	agent, err := NewPortraitAgent(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return portraitGenerator{agent: agent}, nil
}

// NewDiagnostician adapts a diagnosis Eino agent to the exercise application interface.
func NewDiagnostician(ctx context.Context, cfg Config) (exerciseapp.Diagnostician, error) {
	agent, err := NewDiagnosticianAgent(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return exerciseDiagnostician{agent: agent}, nil
}

// NewMathSolver adapts an Eino agent to the exercise math solver interface.
func NewMathSolver(ctx context.Context, cfg Config) (exerciseapp.MathSolver, error) {
	agent, err := NewMathSolverAgent(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return exerciseMathSolver{agent: agent}, nil
}

// NewQuestionParser adapts an Eino agent to the question parser interface.
func NewQuestionParser(ctx context.Context, cfg Config) (questionapp.Parser, error) {
	agent, err := NewQuestionParserAgent(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return questionParser{agent: agent}, nil
}

func newChatModelAgent(ctx context.Context, cfg Config, spec chatAgentSpec) (*Agent, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	temperature := float32(cfg.Temperature)
	modelConfig := &einoopenai.ChatModelConfig{
		APIKey:      strings.TrimSpace(cfg.APIKey),
		BaseURL:     strings.TrimSpace(cfg.BaseURL),
		Model:       strings.TrimSpace(cfg.Model),
		Timeout:     cfg.Timeout,
		Temperature: &temperature,
	}
	if cfg.MaxTokens > 0 {
		modelConfig.MaxTokens = &cfg.MaxTokens
	}
	if cfg.TopP != nil {
		topP := float32(*cfg.TopP)
		modelConfig.TopP = &topP
	}
	chatModel, err := einoopenai.NewChatModel(ctx, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("create Eino chat model: %w", err)
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          spec.name,
		Description:   spec.description,
		Instruction:   spec.instruction,
		Model:         chatModel,
		MaxIterations: cfg.MaxIterations,
	})
	if err != nil {
		return nil, fmt.Errorf("create Eino %s agent: %w", spec.name, err)
	}
	return &Agent{
		name:   spec.name,
		runner: adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent}),
	}, nil
}

// Generate resolves a Tutor Agent configuration and delegates to an Eino ChatModelAgent.
func (a *ConfigurableAgent) Generate(ctx context.Context, input sessionapp.ChatAgentInput) (sessionapp.ChatAgentOutput, error) {
	if a == nil {
		return sessionapp.ChatAgentOutput{}, errors.New("configurable Eino agent is nil")
	}
	cfg := a.fallback
	if a.provider != nil {
		runtime, ok, err := a.provider.RuntimeConfig(ctx, "tutor")
		if err != nil {
			return sessionapp.ChatAgentOutput{}, fmt.Errorf("load tutor runtime config: %w", err)
		}
		if ok {
			cfg = Config{
				Enabled:       true,
				BaseURL:       runtime.BaseURL,
				APIKey:        runtime.APIKey,
				Model:         runtime.Model,
				Timeout:       runtime.Timeout,
				Temperature:   runtime.Temperature,
				MaxTokens:     runtime.MaxTokens,
				TopP:          runtime.TopP,
				MaxIterations: runtime.MaxIterations,
			}
		}
	}
	newAgent := a.newAgent
	if newAgent == nil {
		newAgent = func(ctx context.Context, cfg Config) (sessionapp.ChatAgent, error) {
			return NewTutorAgent(ctx, cfg)
		}
	}
	agent, err := newAgent(ctx, cfg)
	if err != nil {
		return sessionapp.ChatAgentOutput{}, err
	}
	return agent.Generate(ctx, input)
}

// GeneratePortrait resolves a Portrait Agent configuration and generates portrait content.
func (g *ConfigurablePortraitGenerator) GeneratePortrait(ctx context.Context, input portraitapp.GeneratorInput) (string, error) {
	if g == nil {
		return "", errors.New("configurable Eino portrait generator is nil")
	}
	cfg := g.fallback
	if g.provider != nil {
		runtime, ok, err := g.provider.RuntimeConfig(ctx, "portrait")
		if err != nil {
			return "", fmt.Errorf("load portrait runtime config: %w", err)
		}
		if ok {
			cfg = configFromRuntime(runtime)
		}
	}
	newGenerator := g.newGenerator
	if newGenerator == nil {
		newGenerator = func(ctx context.Context, cfg Config) (portraitapp.Generator, error) {
			return NewPortraitGenerator(ctx, cfg)
		}
	}
	generator, err := newGenerator(ctx, cfg)
	if err != nil {
		return "", err
	}
	return generator.GeneratePortrait(ctx, input)
}

// Diagnose resolves a Diagnostician Agent configuration and generates structured diagnosis.
func (d *ConfigurableDiagnostician) Diagnose(ctx context.Context, input exerciseapp.DiagnosisInput) (exerciseapp.DiagnosisDetail, error) {
	if d == nil {
		return exerciseapp.DiagnosisDetail{}, errors.New("configurable Eino diagnostician is nil")
	}
	cfg := d.fallback
	if d.provider != nil {
		runtime, ok, err := d.provider.RuntimeConfig(ctx, "diagnostician")
		if err != nil {
			return exerciseapp.DiagnosisDetail{}, fmt.Errorf("load diagnostician runtime config: %w", err)
		}
		if ok {
			cfg = configFromRuntime(runtime)
		}
	}
	newDiagnostician := d.newDiagnostician
	if newDiagnostician == nil {
		newDiagnostician = func(ctx context.Context, cfg Config) (exerciseapp.Diagnostician, error) {
			return NewDiagnostician(ctx, cfg)
		}
	}
	diagnostician, err := newDiagnostician(ctx, cfg)
	if err != nil {
		return exerciseapp.DiagnosisDetail{}, err
	}
	return diagnostician.Diagnose(ctx, input)
}

// CheckAnswer resolves a Math Solver Agent configuration and compares answers.
func (s *ConfigurableMathSolver) CheckAnswer(ctx context.Context, input exerciseapp.AnswerCheckInput) (exerciseapp.AnswerCheckResult, error) {
	if s == nil {
		return exerciseapp.AnswerCheckResult{}, errors.New("configurable Eino math solver is nil")
	}
	cfg := s.fallback
	if s.provider != nil {
		runtime, ok, err := s.provider.RuntimeConfig(ctx, "math_solver")
		if err != nil {
			return exerciseapp.AnswerCheckResult{}, fmt.Errorf("load math_solver runtime config: %w", err)
		}
		if ok {
			cfg = configFromRuntime(runtime)
		}
	}
	newSolver := s.newSolver
	if newSolver == nil {
		newSolver = func(ctx context.Context, cfg Config) (exerciseapp.MathSolver, error) {
			return NewMathSolver(ctx, cfg)
		}
	}
	solver, err := newSolver(ctx, cfg)
	if err != nil {
		return exerciseapp.AnswerCheckResult{}, err
	}
	return solver.CheckAnswer(ctx, input)
}

// ParseQuestions resolves a Question Parser Agent configuration and extracts questions.
func (p *ConfigurableQuestionParser) ParseQuestions(ctx context.Context, input questionapp.ParserInput) (questionapp.AIParseResponse, error) {
	if p == nil {
		return questionapp.AIParseResponse{}, errors.New("configurable Eino question parser is nil")
	}
	cfg := p.fallback
	if p.provider != nil {
		runtime, ok, err := p.provider.RuntimeConfig(ctx, "question_parser")
		if err != nil {
			return questionapp.AIParseResponse{}, fmt.Errorf("load question_parser runtime config: %w", err)
		}
		if ok {
			cfg = configFromRuntime(runtime)
		}
	}
	newParser := p.newParser
	if newParser == nil {
		newParser = func(ctx context.Context, cfg Config) (questionapp.Parser, error) {
			return NewQuestionParser(ctx, cfg)
		}
	}
	parser, err := newParser(ctx, cfg)
	if err != nil {
		return questionapp.AIParseResponse{}, err
	}
	return parser.ParseQuestions(ctx, input)
}

// Generate runs the tutor agent and collects the final assistant message.
func (a *Agent) Generate(ctx context.Context, input sessionapp.ChatAgentInput) (sessionapp.ChatAgentOutput, error) {
	if a == nil || a.runner == nil {
		return sessionapp.ChatAgentOutput{}, errors.New("Eino tutor agent is not configured")
	}
	events := a.runner.Run(ctx, toMessages(input))
	content := ""
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return sessionapp.ChatAgentOutput{}, fmt.Errorf("run Eino tutor agent: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return sessionapp.ChatAgentOutput{}, fmt.Errorf("read Eino tutor output: %w", err)
		}
		if strings.TrimSpace(message.Content) != "" {
			content = message.Content
		}
	}
	if strings.TrimSpace(content) == "" {
		return sessionapp.ChatAgentOutput{}, errors.New("Eino tutor agent returned empty content")
	}
	return sessionapp.ChatAgentOutput{Agent: a.name, Content: content}, nil
}

type portraitGenerator struct {
	agent sessionapp.ChatAgent
}

func (g portraitGenerator) GeneratePortrait(ctx context.Context, input portraitapp.GeneratorInput) (string, error) {
	if g.agent == nil {
		return "", errors.New("Eino portrait agent is not configured")
	}
	output, err := g.agent.Generate(ctx, sessionapp.ChatAgentInput{
		Message: portraitPrompt(input),
	})
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(output.Content)
	if content == "" {
		return "", errors.New("Eino portrait agent returned empty content")
	}
	return content, nil
}

type exerciseDiagnostician struct {
	agent sessionapp.ChatAgent
}

func (d exerciseDiagnostician) Diagnose(ctx context.Context, input exerciseapp.DiagnosisInput) (exerciseapp.DiagnosisDetail, error) {
	if d.agent == nil {
		return exerciseapp.DiagnosisDetail{}, errors.New("Eino diagnostician agent is not configured")
	}
	output, err := d.agent.Generate(ctx, sessionapp.ChatAgentInput{
		Message: diagnosticianPrompt(input),
	})
	if err != nil {
		return exerciseapp.DiagnosisDetail{}, err
	}
	return parseDiagnosisJSON(output.Content)
}

type exerciseMathSolver struct {
	agent sessionapp.ChatAgent
}

func (s exerciseMathSolver) CheckAnswer(ctx context.Context, input exerciseapp.AnswerCheckInput) (exerciseapp.AnswerCheckResult, error) {
	if s.agent == nil {
		return exerciseapp.AnswerCheckResult{}, errors.New("Eino math solver agent is not configured")
	}
	output, err := s.agent.Generate(ctx, sessionapp.ChatAgentInput{
		Message: mathSolverPrompt(input),
	})
	if err != nil {
		return exerciseapp.AnswerCheckResult{}, err
	}
	return parseAnswerCheckJSON(output.Content)
}

type questionParser struct {
	agent sessionapp.ChatAgent
}

func (p questionParser) ParseQuestions(ctx context.Context, input questionapp.ParserInput) (questionapp.AIParseResponse, error) {
	if p.agent == nil {
		return questionapp.AIParseResponse{}, errors.New("Eino question parser agent is not configured")
	}
	output, err := p.agent.Generate(ctx, sessionapp.ChatAgentInput{
		Message: questionParserPrompt(input),
	})
	if err != nil {
		return questionapp.AIParseResponse{}, err
	}
	return parseQuestionParseJSON(output.Content)
}

func configFromRuntime(runtime adminaiconfigapp.RuntimeConfig) Config {
	return Config{
		Enabled:       true,
		BaseURL:       runtime.BaseURL,
		APIKey:        runtime.APIKey,
		Model:         runtime.Model,
		Timeout:       runtime.Timeout,
		Temperature:   runtime.Temperature,
		MaxTokens:     runtime.MaxTokens,
		TopP:          runtime.TopP,
		MaxIterations: runtime.MaxIterations,
	}
}

func validateConfig(cfg Config) error {
	if !cfg.Enabled {
		return errors.New("Eino agent is disabled")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New("Eino API key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return errors.New("Eino model is required")
	}
	if cfg.Timeout <= 0 {
		return errors.New("Eino timeout must be greater than zero")
	}
	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		return errors.New("Eino temperature must be between 0 and 2")
	}
	if cfg.MaxTokens < 0 {
		return errors.New("Eino max tokens must be zero or greater")
	}
	if cfg.MaxIterations <= 0 {
		return errors.New("Eino max iterations must be greater than zero")
	}
	return nil
}

func toMessages(input sessionapp.ChatAgentInput) []adk.Message {
	messages := make([]adk.Message, 0, len(input.History)+1)
	for _, history := range input.History {
		content := strings.TrimSpace(history.Content)
		if content == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(history.Role)) {
		case "assistant":
			messages = append(messages, schema.AssistantMessage(content, nil))
		case "user":
			messages = append(messages, schema.UserMessage(content))
		}
	}
	userMessage := strings.TrimSpace(input.Message)
	if len(input.Attachments) > 0 {
		userMessage += "\n\n附件：" + strings.Join(input.Attachments, "、")
	}
	messages = append(messages, schema.UserMessage(userMessage))
	return messages
}

func portraitPrompt(input portraitapp.GeneratorInput) string {
	profile := input.Profile
	var builder strings.Builder
	builder.WriteString("请基于以下学习数据生成一份学生画像报告。\n\n")
	builder.WriteString("硬性要求：\n")
	builder.WriteString("- 使用中文 Markdown。\n")
	builder.WriteString("- 不要输出 JSON。\n")
	builder.WriteString("- 不要编造输入中没有的身份、课程、考试或教师评价。\n")
	builder.WriteString("- 保留关键数字，并在数据不足时说明置信度有限。\n\n")
	builder.WriteString("学习统计：\n")
	builder.WriteString(fmt.Sprintf("- 学生 ID: %s\n", strings.TrimSpace(profile.StudentID)))
	builder.WriteString(fmt.Sprintf("- 总练习次数: %d\n", profile.TotalExercises))
	builder.WriteString(fmt.Sprintf("- 正确次数: %d\n", profile.CorrectCount))
	builder.WriteString(fmt.Sprintf("- 总学习时长: %d 分钟\n", profile.TotalStudyTimeMinutes))
	builder.WriteString(fmt.Sprintf("- 偏好难度: %.2f\n", profile.PreferredDifficulty))
	builder.WriteString(fmt.Sprintf("- 学习节奏系数: %.2f\n", profile.LearningPace))
	appendFloatMap(&builder, "知识点掌握度", profile.MasteryVector)
	appendFloatMap(&builder, "错误倾向", profile.ErrorTendency)
	if len(profile.RecentConcepts) > 0 {
		builder.WriteString("\n近期学习重点：\n")
		for _, concept := range profile.RecentConcepts {
			concept = strings.TrimSpace(concept)
			if concept != "" {
				builder.WriteString("- " + concept + "\n")
			}
		}
	}
	builder.WriteString("\n模板基线报告（可作为事实依据和兜底结构）：\n")
	builder.WriteString(strings.TrimSpace(input.FallbackContent))
	builder.WriteString("\n\n请生成最终画像报告。")
	return builder.String()
}

func appendFloatMap(builder *strings.Builder, title string, values map[string]float64) {
	if len(values) == 0 {
		return
	}
	items := make([]struct {
		key   string
		value float64
	}, 0, len(values))
	for key, value := range values {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			items = append(items, struct {
				key   string
				value float64
			}{key: trimmed, value: value})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].key < items[j].key })
	if len(items) == 0 {
		return
	}
	builder.WriteString("\n" + title + "：\n")
	for _, item := range items {
		builder.WriteString(fmt.Sprintf("- %s: %.4f\n", item.key, item.value))
	}
}

func diagnosticianPrompt(input exerciseapp.DiagnosisInput) string {
	var builder strings.Builder
	builder.WriteString("请诊断以下高等数学作答错误，并只返回 JSON。\n\n")
	builder.WriteString("JSON 字段：error_type、error_subtype、taxonomy_code、error_description、error_step_index、severity、suggestion、related_concepts。\n")
	builder.WriteString("题目信息：\n")
	builder.WriteString(fmt.Sprintf("- 题目 ID: %s\n", input.Exercise.ID))
	builder.WriteString(fmt.Sprintf("- 标题: %s\n", input.Exercise.Title))
	builder.WriteString(fmt.Sprintf("- 内容: %s\n", input.Exercise.Body))
	builder.WriteString(fmt.Sprintf("- 难度: %.2f\n", input.Exercise.Difficulty))
	if len(input.Exercise.ConceptIDs) > 0 {
		builder.WriteString("- 知识点: " + strings.Join(input.Exercise.ConceptIDs, "、") + "\n")
	}
	builder.WriteString("作答信息：\n")
	builder.WriteString(fmt.Sprintf("- 学生答案: %s\n", input.StudentAnswer))
	builder.WriteString(fmt.Sprintf("- 标准答案: %s\n", input.CorrectAnswer))
	builder.WriteString(fmt.Sprintf("- 本地判题理由: %s\n", input.Check.Reason))
	builder.WriteString(fmt.Sprintf("- 本地判题置信度: %.2f\n", input.Check.Confidence))
	builder.WriteString(fmt.Sprintf("- 图片答案: %v\n", input.ImageOnly))
	if len(input.AnswerSteps) > 0 {
		builder.WriteString("学生步骤：\n")
		for index, step := range input.AnswerSteps {
			step = strings.TrimSpace(step)
			if step != "" {
				builder.WriteString(fmt.Sprintf("%d. %s\n", index+1, step))
			}
		}
	}
	builder.WriteString("本地兜底诊断：\n")
	builder.WriteString(diagnosisAsJSON(input.Fallback))
	return builder.String()
}

func mathSolverPrompt(input exerciseapp.AnswerCheckInput) string {
	var builder strings.Builder
	builder.WriteString("请比较学生答案与标准答案是否数学等价，并只返回 JSON。\n\n")
	builder.WriteString("JSON 字段：is_correct、reason、confidence。\n")
	builder.WriteString("比较上下文：\n")
	builder.WriteString(fmt.Sprintf("- 答案类型: %s\n", strings.TrimSpace(input.AnswerType)))
	builder.WriteString(fmt.Sprintf("- 学生答案: %s\n", strings.TrimSpace(input.StudentAnswer)))
	builder.WriteString(fmt.Sprintf("- 标准答案: %s\n", strings.TrimSpace(input.CorrectAnswer)))
	builder.WriteString("本地兜底判定：\n")
	body, _ := json.Marshal(map[string]any{
		"is_correct": input.Fallback.IsCorrect,
		"reason":     input.Fallback.Reason,
		"confidence": input.Fallback.Confidence,
	})
	builder.WriteString(string(body))
	return builder.String()
}

func questionParserPrompt(input questionapp.ParserInput) string {
	var builder strings.Builder
	builder.WriteString("请从以下原始文本解析高等数学题目，并只返回 JSON。\n\n")
	builder.WriteString("JSON 顶层格式：{\"questions\":[{\"title\":\"\",\"body\":\"\",\"type\":\"short_answer\",\"difficulty\":0.5,\"answer\":\"\",\"answer_type\":\"expression\",\"options\":[],\"hints\":[],\"solution_steps\":[],\"tags\":[]}]}\n")
	builder.WriteString("如果无法确定答案，请 answer 留空；不要编造标准答案。\n\n")
	for index, text := range input.RawTexts {
		builder.WriteString(fmt.Sprintf("原始文本 %d：\n", index+1))
		builder.WriteString(strings.TrimSpace(text))
		builder.WriteString("\n\n")
	}
	builder.WriteString("本地兜底解析：\n")
	body, _ := json.Marshal(input.Fallback)
	builder.WriteString(string(body))
	return builder.String()
}

func diagnosisAsJSON(diagnosis exerciseapp.DiagnosisDetail) string {
	payload := map[string]any{
		"error_type":        diagnosis.ErrorType,
		"error_subtype":     diagnosis.ErrorSubtype,
		"taxonomy_code":     diagnosis.TaxonomyCode,
		"error_description": diagnosis.ErrorDescription,
		"error_step_index":  diagnosis.ErrorStepIndex,
		"severity":          diagnosis.Severity,
		"suggestion":        diagnosis.Suggestion,
		"related_concepts":  diagnosis.RelatedConcepts,
	}
	body, _ := json.Marshal(payload)
	return string(body)
}

func parseDiagnosisJSON(content string) (exerciseapp.DiagnosisDetail, error) {
	content = stripJSONFence(content)
	var payload struct {
		ErrorType        *string  `json:"error_type"`
		ErrorSubtype     string   `json:"error_subtype"`
		TaxonomyCode     string   `json:"taxonomy_code"`
		ErrorDescription string   `json:"error_description"`
		ErrorStepIndex   *int     `json:"error_step_index"`
		Severity         string   `json:"severity"`
		Suggestion       string   `json:"suggestion"`
		RelatedConcepts  []string `json:"related_concepts"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return exerciseapp.DiagnosisDetail{}, fmt.Errorf("parse diagnostician JSON: %w", err)
	}
	diagnosis := exerciseapp.DiagnosisDetail{
		ErrorType:        payload.ErrorType,
		ErrorSubtype:     strings.TrimSpace(payload.ErrorSubtype),
		TaxonomyCode:     strings.TrimSpace(payload.TaxonomyCode),
		ErrorDescription: strings.TrimSpace(payload.ErrorDescription),
		ErrorStepIndex:   payload.ErrorStepIndex,
		Severity:         strings.ToLower(strings.TrimSpace(payload.Severity)),
		Suggestion:       strings.TrimSpace(payload.Suggestion),
		RelatedConcepts:  trimStringSlice(payload.RelatedConcepts),
	}
	if err := validateDiagnosis(diagnosis); err != nil {
		return exerciseapp.DiagnosisDetail{}, err
	}
	return diagnosis, nil
}

func parseAnswerCheckJSON(content string) (exerciseapp.AnswerCheckResult, error) {
	content = stripJSONFence(content)
	var payload struct {
		IsCorrect  *bool   `json:"is_correct"`
		Reason     string  `json:"reason"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return exerciseapp.AnswerCheckResult{}, fmt.Errorf("parse math solver JSON: %w", err)
	}
	if payload.IsCorrect == nil {
		return exerciseapp.AnswerCheckResult{}, errors.New("math solver JSON missing is_correct")
	}
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		return exerciseapp.AnswerCheckResult{}, errors.New("math solver JSON missing reason")
	}
	if payload.Confidence < 0 || payload.Confidence > 1 {
		return exerciseapp.AnswerCheckResult{}, errors.New("math solver JSON confidence out of range")
	}
	return exerciseapp.AnswerCheckResult{
		IsCorrect:  *payload.IsCorrect,
		Reason:     reason,
		Confidence: payload.Confidence,
	}, nil
}

func parseQuestionParseJSON(content string) (questionapp.AIParseResponse, error) {
	content = stripJSONFence(content)
	var response questionapp.AIParseResponse
	if err := json.Unmarshal([]byte(content), &response); err != nil {
		return questionapp.AIParseResponse{}, fmt.Errorf("parse question parser JSON: %w", err)
	}
	if len(response.Questions) == 0 || len(response.Questions) > 20 {
		return questionapp.AIParseResponse{}, errors.New("question parser JSON returned invalid question count")
	}
	for _, item := range response.Questions {
		if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Body) == "" {
			return questionapp.AIParseResponse{}, errors.New("question parser JSON missing required question fields")
		}
	}
	return response, nil
}

func stripJSONFence(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}

func validateDiagnosis(diagnosis exerciseapp.DiagnosisDetail) error {
	if diagnosis.ErrorType == nil || strings.TrimSpace(*diagnosis.ErrorType) == "" {
		return errors.New("diagnostician JSON missing error_type")
	}
	errorType := strings.ToLower(strings.TrimSpace(*diagnosis.ErrorType))
	switch errorType {
	case "conceptual", "procedural", "logical", "symbolic":
	default:
		return fmt.Errorf("diagnostician JSON has unsupported error_type %q", errorType)
	}
	if diagnosis.Severity != "low" && diagnosis.Severity != "medium" && diagnosis.Severity != "high" {
		return errors.New("diagnostician JSON has unsupported severity")
	}
	if diagnosis.ErrorSubtype == "" || diagnosis.ErrorDescription == "" || diagnosis.Suggestion == "" {
		return errors.New("diagnostician JSON missing required text fields")
	}
	expectedCode := map[string]string{
		"conceptual": "C-Type",
		"procedural": "P-Type",
		"logical":    "L-Type",
		"symbolic":   "S-Type",
	}[errorType]
	if diagnosis.TaxonomyCode != "" && diagnosis.TaxonomyCode != expectedCode {
		return fmt.Errorf("diagnostician JSON taxonomy_code %q does not match %s", diagnosis.TaxonomyCode, errorType)
	}
	*diagnosis.ErrorType = errorType
	return nil
}

func trimStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
