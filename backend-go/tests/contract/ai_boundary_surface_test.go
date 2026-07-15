package contract

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestOCRAndMathBoundariesStayFailClosed(t *testing.T) {
	root := repoRoot(t)
	expectations := []struct {
		file     string
		required []string
	}{
		{
			file: "backend-go/internal/application/answerocr/service.go",
			required: []string{
				"type ImageLoader interface",
				"type Recognizer interface",
				"MinimumConfidence",
				"ErrUnreadable",
			},
		},
		{
			file: "backend-go/internal/application/exercise/service.go",
			required: []string{
				"WithAnswerOCR",
				"WithSolutionSolver",
				"DecisionIndeterminate",
				"solver_verified",
				"FailureVerificationFailed",
				"Recorded:           false",
				"submissionExercise(ctx, s.repo",
			},
		},
		{
			file: "backend-go/internal/adapter/llm/einoagent/agent.go",
			required: []string{
				"solution_generation",
				"parseSolutionJSON",
				"SolutionStatusIndeterminate",
			},
		},
		{
			file:     "backend-go/cmd/api/main.go",
			required: []string{"WithSolutionSolver(mathSolver)"},
		},
		{
			file: "backend-go/internal/adapter/http/exercise/handler.go",
			required: []string{
				"OCR_UNREADABLE",
				"OCR_UNAVAILABLE",
				"MATH_SOLVER_UNAVAILABLE",
				"OCR_RATE_LIMITED",
			},
		},
		{
			file: "backend-go/internal/adapter/llm/einoagent/ocr.go",
			required: []string{
				"UserInputMultiContent",
				"Base64Data",
				"NewConfigurableAnswerOCR",
			},
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.file, func(t *testing.T) {
			source := readFile(t, filepath.Join(root, expectation.file))
			for _, required := range expectation.required {
				if !strings.Contains(source, required) {
					t.Fatalf("%s must keep explicit remaining AI boundary marker %q", expectation.file, required)
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
				"portraitInstruction",
				"diagnosticianInstruction",
				"mathSolverInstruction",
				"questionParserInstruction",
				"questionGeneratorInstruction",
				"NewConfigurablePortraitGenerator",
				"NewConfigurableDiagnostician",
				"NewConfigurableMathSolver",
				"NewConfigurableQuestionParser",
				"NewConfigurableQuestionGenerator",
			},
		},
		{
			file: "backend-go/internal/application/question/service.go",
			required: []string{
				"type Parser interface",
				"WithParser",
				"deterministic fallback",
			},
		},
		{
			file: "backend-go/internal/application/portrait/service.go",
			required: []string{
				"type Generator interface",
				"WithGenerator",
				"buildPortraitContent",
			},
		},
		{
			file: "backend-go/internal/application/exercise/service.go",
			required: []string{
				"type MathSolver interface",
				"SolverAnswerChecker",
				"type Diagnostician interface",
				"WithDiagnostician",
				"type QuestionGenerator interface",
				"WithQuestionGenerator",
				"NormalizedAnswerChecker is the deterministic local checker used before the Math Solver agent",
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
				"adminaiconfigapp.NewService",
				"adapterpostgres.NewAdminAIConfigRepository",
				"einoagent.NewConfigurableTutorAgent",
				"einoagent.NewConfigurablePortraitGenerator",
				"einoagent.NewConfigurableDiagnostician",
				"einoagent.NewConfigurableMathSolver",
				"einoagent.NewConfigurableAnswerOCR",
				"einoagent.NewConfigurableQuestionParser",
				"einoagent.NewConfigurableQuestionGenerator",
				"sessionapp.WithChatAgent",
				"portraitapp.WithGenerator",
				"exerciseapp.WithDiagnostician",
				"exerciseapp.WithAnswerOCR",
				"exerciseapp.WithQuestionGenerator",
				"exerciseapp.SolverAnswerChecker",
				"questionapp.WithParser",
			},
		},
		{
			file: "backend-go/internal/adapter/http/adminaiconfig/handler.go",
			required: []string{
				"CreateProviderWithModels",
				"UpdateAgentConfig",
				"FetchModelsByCredentials",
			},
		},
		{
			file: "backend-go/internal/application/adminaiconfig/service.go",
			required: []string{
				"type RuntimeConfig struct",
				"RuntimeConfig(ctx context.Context, agentType string)",
				"FetchModelsByCredentials",
				`"ocr"`,
				"question_generator",
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
