package einoagent

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	adminaiconfigapp "mathstudy/backend-go/internal/application/adminaiconfig"
	answerocrapp "mathstudy/backend-go/internal/application/answerocr"
)

func TestAnswerOCRSendsBase64MultimodalImageAndParsesResult(t *testing.T) {
	chatModel := &fakeOCRChatModel{response: &schema.Message{
		Role:    schema.Assistant,
		Content: `{"status":"ok","answer_latex":"x + 1","confidence":0.94,"reason":"最终一行清晰"}`,
	}}
	recognizer := answerOCR{model: chatModel}
	imageData := []byte("real-image-bytes")

	result, err := recognizer.Recognize(context.Background(), answerocrapp.RecognizeInput{
		Image:      answerocrapp.Image{Data: imageData, MIMEType: "image/png"},
		AnswerType: "expression",
	})
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if result.Status != "ok" || result.AnswerLatex != "x + 1" || result.Confidence != 0.94 {
		t.Fatalf("result = %#v", result)
	}
	if len(chatModel.input) != 2 || chatModel.input[0].Role != schema.System || chatModel.input[1].Role != schema.User {
		t.Fatalf("messages = %#v", chatModel.input)
	}
	parts := chatModel.input[1].UserInputMultiContent
	if len(parts) != 2 || !strings.Contains(parts[0].Text, "expression") || parts[1].Image == nil {
		t.Fatalf("multimodal parts = %#v", parts)
	}
	if parts[1].Image.Base64Data == nil || *parts[1].Image.Base64Data != base64.StdEncoding.EncodeToString(imageData) {
		t.Fatalf("image base64 = %#v", parts[1].Image)
	}
	if parts[1].Image.MIMEType != "image/png" || parts[1].Image.Detail != schema.ImageURLDetailHigh {
		t.Fatalf("image part = %#v", parts[1].Image)
	}
}

func TestAnswerOCRReadsTextFromAssistantMultimodalOutput(t *testing.T) {
	chatModel := &fakeOCRChatModel{response: &schema.Message{
		Role: schema.Assistant,
		AssistantGenMultiContent: []schema.MessageOutputPart{{
			Type: schema.ChatMessagePartTypeText,
			Text: `{"status":"unreadable","answer_latex":"","confidence":0.2,"reason":"图片模糊"}`,
		}},
	}}
	result, err := (answerOCR{model: chatModel}).Recognize(context.Background(), answerocrapp.RecognizeInput{
		Image: answerocrapp.Image{Data: []byte("image"), MIMEType: "image/jpeg"},
	})
	if err != nil || result.Status != "unreadable" || result.Reason != "图片模糊" {
		t.Fatalf("Recognize() = %#v, %v", result, err)
	}
}

func TestAnswerOCRRejectsInvalidImageBeforeModel(t *testing.T) {
	chatModel := &fakeOCRChatModel{}
	recognizer := answerOCR{model: chatModel}
	for _, image := range []answerocrapp.Image{
		{},
		{Data: []byte("svg"), MIMEType: "image/svg+xml"},
		{Data: []byte("RIFF....WEBP"), MIMEType: "image/webp"},
		{Data: make([]byte, answerocrapp.MaxImageSize+1), MIMEType: "image/png"},
	} {
		_, err := recognizer.Recognize(context.Background(), answerocrapp.RecognizeInput{Image: image})
		if !errors.Is(err, answerocrapp.ErrInvalidImage) {
			t.Fatalf("Recognize(%#v) error = %v, want ErrInvalidImage", image, err)
		}
	}
	if chatModel.calls != 0 {
		t.Fatalf("model calls = %d", chatModel.calls)
	}
}

func TestParseAnswerOCRJSONStrictValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "markdown fence", content: "```json\n{\"status\":\"ok\",\"answer_latex\":\"x\",\"confidence\":1,\"reason\":\"clear\"}\n```"},
		{name: "unknown field", content: `{"status":"ok","answer_latex":"x","confidence":1,"reason":"clear","extra":true}`},
		{name: "trailing JSON", content: `{"status":"ok","answer_latex":"x","confidence":1,"reason":"clear"}{}`},
		{name: "duplicate field", content: `{"status":"ok","answer_latex":"x","answer_latex":"y","confidence":1,"reason":"clear"}`},
		{name: "missing status", content: `{"answer_latex":"x","confidence":1,"reason":"clear"}`},
		{name: "missing answer", content: `{"status":"unreadable","confidence":0.2,"reason":"blurred"}`},
		{name: "missing confidence", content: `{"status":"ok","answer_latex":"x","reason":"clear"}`},
		{name: "missing reason", content: `{"status":"ok","answer_latex":"x","confidence":1,"reason":""}`},
		{name: "null status", content: `{"status":null,"answer_latex":"x","confidence":1,"reason":"clear"}`},
		{name: "null answer", content: `{"status":"unreadable","answer_latex":null,"confidence":0.2,"reason":"blurred"}`},
		{name: "null confidence", content: `{"status":"unreadable","answer_latex":"","confidence":null,"reason":"blurred"}`},
		{name: "null reason", content: `{"status":"unreadable","answer_latex":"","confidence":0.2,"reason":null}`},
		{name: "wrong field type", content: `{"status":"ok","answer_latex":42,"confidence":1,"reason":"clear"}`},
		{name: "invalid status", content: `{"status":"maybe","answer_latex":"x","confidence":1,"reason":"clear"}`},
		{name: "ok without answer", content: `{"status":"ok","answer_latex":"","confidence":1,"reason":"clear"}`},
		{name: "unreadable with answer", content: `{"status":"unreadable","answer_latex":"x","confidence":0.2,"reason":"blurred"}`},
		{name: "confidence out of range", content: `{"status":"ok","answer_latex":"x","confidence":1.1,"reason":"clear"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseAnswerOCRJSON(tt.content); err == nil {
				t.Fatal("parseAnswerOCRJSON() error = nil")
			}
		})
	}
}

func TestParseAnswerOCRJSONAcceptsExactUnreadableResult(t *testing.T) {
	result, err := parseAnswerOCRJSON(`{"status":"unreadable","answer_latex":"","confidence":0.25,"reason":"没有明确最终答案"}`)
	if err != nil {
		t.Fatalf("parseAnswerOCRJSON() error = %v", err)
	}
	if result.Status != "unreadable" || result.AnswerLatex != "" || result.Confidence != 0.25 {
		t.Fatalf("result = %#v", result)
	}
}

func TestConfigurableAnswerOCRUsesOCRRuntimeConfig(t *testing.T) {
	provider := &fakeOCRRuntimeConfigProvider{
		runtime: adminaiconfigapp.RuntimeConfig{
			BaseURL:       "https://api.example.com",
			APIKey:        "ocr-key",
			Model:         "vision-model",
			Temperature:   0,
			MaxTokens:     300,
			Timeout:       time.Second,
			MaxIterations: 1,
		},
		ok: true,
	}
	recognizer := NewConfigurableAnswerOCR(provider, Config{Enabled: false})
	var captured Config
	recognizer.newRecognizer = func(_ context.Context, cfg Config) (answerocrapp.Recognizer, error) {
		captured = cfg
		return fakeAnswerOCRRecognizer{result: answerocrapp.Result{Status: "ok", AnswerLatex: "2x", Confidence: 0.9, Reason: "clear"}}, nil
	}

	result, err := recognizer.Recognize(context.Background(), answerocrapp.RecognizeInput{})
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if !provider.called || provider.agentType != "ocr" || result.AnswerLatex != "2x" {
		t.Fatalf("provider=%#v result=%#v", provider, result)
	}
	if captured.Model != "vision-model" || captured.APIKey != "ocr-key" || captured.MaxTokens != 300 {
		t.Fatalf("captured config = %#v", captured)
	}
}

func TestNewAnswerOCRRejectsDisabledConfig(t *testing.T) {
	if _, err := NewAnswerOCR(context.Background(), Config{Enabled: false}); err == nil {
		t.Fatal("NewAnswerOCR(disabled) error = nil")
	}
}

type fakeOCRChatModel struct {
	response *schema.Message
	input    []*schema.Message
	err      error
	calls    int
}

func (m *fakeOCRChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.calls++
	m.input = input
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *fakeOCRChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not implemented")
}

type fakeOCRRuntimeConfigProvider struct {
	runtime   adminaiconfigapp.RuntimeConfig
	ok        bool
	called    bool
	agentType string
	err       error
}

func (p *fakeOCRRuntimeConfigProvider) RuntimeConfig(_ context.Context, agentType string) (adminaiconfigapp.RuntimeConfig, bool, error) {
	p.called = true
	p.agentType = agentType
	if p.err != nil {
		return adminaiconfigapp.RuntimeConfig{}, false, p.err
	}
	return p.runtime, p.ok, nil
}

type fakeAnswerOCRRecognizer struct {
	result answerocrapp.Result
	err    error
}

func (r fakeAnswerOCRRecognizer) Recognize(context.Context, answerocrapp.RecognizeInput) (answerocrapp.Result, error) {
	if r.err != nil {
		return answerocrapp.Result{}, r.err
	}
	return r.result, nil
}
