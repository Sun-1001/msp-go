package einoagent

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	mathsolverapp "mathstudy/backend-go/internal/application/mathsolver"
)

func TestLiveMathSolverCoversGeneralProblemFamiliesAndRejectsInvalidSteps(t *testing.T) {
	if os.Getenv("MSP_LIVE_MATH_ACCEPTANCE") != "1" {
		t.Skip("set MSP_LIVE_MATH_ACCEPTANCE=1 with MSP_MATH_ACCEPTANCE_API_KEY and MSP_MATH_ACCEPTANCE_MODEL to run")
	}
	apiKey := strings.TrimSpace(os.Getenv("MSP_MATH_ACCEPTANCE_API_KEY"))
	modelName := strings.TrimSpace(os.Getenv("MSP_MATH_ACCEPTANCE_MODEL"))
	if apiKey == "" || modelName == "" {
		t.Fatal("live Math Solver acceptance requires MSP_MATH_ACCEPTANCE_API_KEY and MSP_MATH_ACCEPTANCE_MODEL")
	}

	solver, err := NewMathSolver(context.Background(), Config{
		Enabled:       true,
		BaseURL:       strings.TrimSpace(os.Getenv("MSP_MATH_ACCEPTANCE_BASE_URL")),
		APIKey:        apiKey,
		Model:         modelName,
		Timeout:       60 * time.Second,
		Temperature:   0,
		MaxTokens:     1_500,
		MaxIterations: 1,
	})
	if err != nil {
		t.Fatalf("create live Math Solver: %v", err)
	}
	solutionSolver, ok := solver.(exerciseapp.SolutionSolver)
	if !ok {
		t.Fatal("live Math Solver does not implement SolutionSolver")
	}
	verifier, ok := solver.(exerciseapp.SolutionVerifier)
	if !ok {
		t.Fatal("live Math Solver does not implement SolutionVerifier")
	}

	tests := []struct {
		name            string
		body            string
		answerType      string
		referenceAnswer string
	}{
		{name: "trigonometric_identity", body: `Simplify sin(x)^2 + cos(x)^2.`, answerType: "expression", referenceAnswer: "1"},
		{name: "limit", body: `Evaluate lim_{x->0} sin(x)/x.`, answerType: "expression", referenceAnswer: "1"},
		{name: "indefinite_integral", body: `Find the indefinite integral of 2*x with respect to x.`, answerType: "expression", referenceAnswer: "x^2+C"},
		{name: "equation_solution_set", body: `Solve x^2-1=0 over the real numbers and return the complete solution set.`, answerType: "text", referenceAnswer: "{-1,1}"},
		{name: "matrix_product", body: `Compute [[1,2],[3,4]] * [[0,1],[1,0]].`, answerType: "text", referenceAnswer: "[[2,1],[4,3]]"},
		{name: "proof", body: `Prove that the square of every odd integer is odd.`, answerType: "text", referenceAnswer: `Let n=2k+1. Then n^2=2*(2k^2+2k)+1, so n^2 is odd.`},
	}

	var trigonometricCandidate exerciseapp.SolutionResult
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exercise := exerciseapp.Exercise{
				ID:     "live-" + test.name,
				Title:  test.name,
				Body:   test.body,
				Status: "PUBLISHED",
				Meta: map[string]any{
					"type":        "short_answer",
					"answer_type": test.answerType,
				},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			candidate, err := solutionSolver.Solve(ctx, exerciseapp.SolutionInput{
				Exercise:   exercise,
				AnswerType: test.answerType,
			})
			if err != nil {
				t.Fatalf("live solve with model %q: %v", modelName, err)
			}
			if candidate.Status != exerciseapp.SolutionStatusSolved || candidate.Confidence < 0.7 || candidate.Answer == "" || len(candidate.Steps) == 0 {
				t.Fatalf("live solution with model %q = %#v", modelName, candidate)
			}
			verification, err := verifier.VerifySolution(ctx, exerciseapp.SolutionVerificationInput{
				Exercise:        exercise,
				CandidateAnswer: candidate.Answer,
				CandidateSteps:  candidate.Steps,
				ReferenceAnswer: test.referenceAnswer,
				AnswerType:      test.answerType,
			})
			if err != nil {
				t.Fatalf("live verification with model %q: %v", modelName, err)
			}
			if verification.Decision != mathsolverapp.DecisionCorrect || verification.Confidence < 0.7 {
				t.Fatalf("live verification with model %q = %#v", modelName, verification)
			}
			if test.name == "trigonometric_identity" {
				trigonometricCandidate = candidate
			}
		})
	}

	badVerification, err := verifier.VerifySolution(context.Background(), exerciseapp.SolutionVerificationInput{
		Exercise: exerciseapp.Exercise{
			ID:    "live-invalid-steps",
			Title: "invalid_steps",
			Body:  `Simplify sin(x)^2 + cos(x)^2.`,
			Meta:  map[string]any{"type": "short_answer", "answer_type": "expression"},
		},
		CandidateAnswer: trigonometricCandidate.Answer,
		CandidateSteps: []string{
			`Assume sin(x)=0 and cos(x)=0 for every x; therefore the expression equals 1.`,
		},
		ReferenceAnswer: "1",
		AnswerType:      "expression",
	})
	if err != nil {
		t.Fatalf("live invalid-step verification with model %q: %v", modelName, err)
	}
	if badVerification.Decision == mathsolverapp.DecisionCorrect {
		t.Fatalf("live verifier accepted invalid steps with model %q: %#v", modelName, badVerification)
	}
}
