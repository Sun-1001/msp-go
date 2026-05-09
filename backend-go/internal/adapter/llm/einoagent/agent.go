package einoagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	sessionapp "mathstudy/backend-go/internal/application/session"
)

const tutorInstruction = `你是高等数学智能学习平台的导师智能体。
目标：用中文给学生提供清晰、分步骤、可执行的辅导。
约束：
- 优先解释思路，不直接跳到结论。
- 对公式使用 LaTeX。
- 如果题目或上下文不足，先说明缺失信息并给出下一步建议。
- 不编造学生画像、课程数据或题库中不存在的信息。`

// Config stores Eino runtime settings for the tutor agent.
type Config struct {
	Enabled       bool
	BaseURL       string
	APIKey        string
	Model         string
	Timeout       time.Duration
	Temperature   float64
	MaxTokens     int
	MaxIterations int
}

// Agent adapts Eino ADK to the session ChatAgent interface.
type Agent struct {
	name   string
	runner *adk.Runner
}

// NewTutorAgent creates an Eino ChatModelAgent backed by an OpenAI-compatible model.
func NewTutorAgent(ctx context.Context, cfg Config) (*Agent, error) {
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
	chatModel, err := einoopenai.NewChatModel(ctx, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("create Eino chat model: %w", err)
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "tutor",
		Description:   "高等数学学习辅导智能体，负责讲解概念、分析解题思路和给出练习建议。",
		Instruction:   tutorInstruction,
		Model:         chatModel,
		MaxIterations: cfg.MaxIterations,
	})
	if err != nil {
		return nil, fmt.Errorf("create Eino tutor agent: %w", err)
	}
	return &Agent{
		name:   "tutor",
		runner: adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent}),
	}, nil
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
