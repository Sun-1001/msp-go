package einoagent

import (
	"strings"
	"testing"
	"time"

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

func TestToMessagesKeepsHistoryAndAttachmentContext(t *testing.T) {
	messages := toMessages(sessionapp.ChatAgentInput{
		Message:     "讲一下洛必达法则",
		Attachments: []string{"/uploads/a.png"},
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
	if !strings.Contains(messages[2].Content, "洛必达法则") || !strings.Contains(messages[2].Content, "/uploads/a.png") {
		t.Fatalf("user message = %q", messages[2].Content)
	}
}
