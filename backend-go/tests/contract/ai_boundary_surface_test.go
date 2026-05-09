package contract

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemainingAIBoundariesStayExplicit(t *testing.T) {
	root := repoRoot(t)
	expectations := []struct {
		file     string
		required []string
	}{
		{
			file: "backend-go/internal/adapter/http/adminaiconfig/handler.go",
			required: []string{
				"P6 AI work is TODO",
				"http.StatusNotImplemented",
				"AI_CONFIG_TODO",
			},
		},
		{
			file: "backend-go/internal/application/question/service.go",
			required: []string{
				"ParseQuestions returns a deterministic shape-compatible parse fallback",
				"LLM extraction remains a P6 TODO",
			},
		},
		{
			file: "backend-go/internal/application/portrait/service.go",
			required: []string{
				"LLM portrait quality remains a P6 TODO",
				"buildPortraitContent",
			},
		},
		{
			file: "backend-go/internal/application/exercise/service.go",
			required: []string{
				"OCR 判题能力将在 AI 迁移阶段恢复",
				"NormalizedAnswerChecker is a deterministic local checker used before P6 AI parity",
			},
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.file, func(t *testing.T) {
			source := readFile(t, filepath.Join(root, expectation.file))
			for _, required := range expectation.required {
				if !strings.Contains(source, required) {
					t.Fatalf("%s must keep explicit AI TODO boundary marker %q", expectation.file, required)
				}
			}
		})
	}
}

func TestSessionChatWiresEinoAgentRuntime(t *testing.T) {
	root := repoRoot(t)
	expectations := []struct {
		file     string
		required []string
	}{
		{
			file: "backend-go/internal/adapter/llm/einoagent/agent.go",
			required: []string{
				"adk.NewChatModelAgent",
				"einoopenai.NewChatModel",
				"tutorInstruction",
			},
		},
		{
			file: "backend-go/internal/application/session/service.go",
			required: []string{
				"type ChatAgent interface",
				"WithChatAgent",
				"ProcessChat stores the user message",
			},
		},
		{
			file: "backend-go/cmd/api/main.go",
			required: []string{
				"einoagent.NewTutorAgent",
				"sessionapp.WithChatAgent",
			},
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.file, func(t *testing.T) {
			source := readFile(t, filepath.Join(root, expectation.file))
			for _, required := range expectation.required {
				if !strings.Contains(source, required) {
					t.Fatalf("%s must wire Eino agent runtime marker %q", expectation.file, required)
				}
			}
		})
	}
}

func TestGoBackendDoesNotWireLegacyAIWorkflowStacks(t *testing.T) {
	root := repoRoot(t)
	forbidden := []string{
		"langchain",
		"langgraph",
		"litellm",
		"paddleocr",
		"sympy",
		"tesseract",
	}
	for _, relRoot := range []string{"backend-go/cmd", "backend-go/internal"} {
		walkRoot := filepath.Join(root, relRoot)
		if err := filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				t.Fatalf("walk %s: %v", path, err)
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			source := strings.ToLower(readFile(t, path))
			for _, token := range forbidden {
				if strings.Contains(source, token) {
					t.Fatalf("%s must not wire legacy AI workflow stack token %q", path, token)
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("walk %s: %v", walkRoot, err)
		}
	}
}
