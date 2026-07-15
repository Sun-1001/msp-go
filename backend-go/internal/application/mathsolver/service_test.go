package mathsolver

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewComparatorUsesSafeDefaults(t *testing.T) {
	comparator := NewComparator()
	if comparator == nil || !validLimits(comparator.limits) {
		t.Fatalf("NewComparator() = %#v, want valid comparator", comparator)
	}
	result := comparator.Compare(context.Background(), CompareInput{
		StudentAnswer:   " x + 1 ",
		ReferenceAnswer: "x+1",
		Kind:            AnswerKindExpression,
	})
	assertResult(t, result, DecisionCorrect, MethodExpressionExact, ReasonExpressionExactMatch)
}

func TestNewComparatorWithLimitsValidatesEveryLimit(t *testing.T) {
	valid := testLimits()
	if comparator, err := NewComparatorWithLimits(valid); err != nil || comparator == nil {
		t.Fatalf("NewComparatorWithLimits(valid) = %#v, %v", comparator, err)
	}

	tests := []struct {
		name   string
		mutate func(*Limits)
		field  string
	}{
		{name: "input bytes", mutate: func(v *Limits) { v.MaxInputBytes = 0 }, field: "MaxInputBytes"},
		{name: "normalized runes", mutate: func(v *Limits) { v.MaxNormalizedRunes = -1 }, field: "MaxNormalizedRunes"},
		{name: "syntax tokens", mutate: func(v *Limits) { v.MaxSyntaxTokens = 0 }, field: "MaxSyntaxTokens"},
		{name: "nesting depth", mutate: func(v *Limits) { v.MaxNestingDepth = 0 }, field: "MaxNestingDepth"},
		{name: "numeric digits", mutate: func(v *Limits) { v.MaxNumericDigits = 0 }, field: "MaxNumericDigits"},
		{name: "exponent", mutate: func(v *Limits) { v.MaxExponent = 0 }, field: "MaxExponent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := valid
			tt.mutate(&limits)
			comparator, err := NewComparatorWithLimits(limits)
			if err == nil || comparator != nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("NewComparatorWithLimits() = %#v, %v, want %s error", comparator, err, tt.field)
			}
		})
	}
}

func TestCompareRoutesAnswerKindsWithoutCollapsingUncertainty(t *testing.T) {
	comparator := NewComparator()
	tests := []struct {
		name       string
		input      CompareInput
		decision   Decision
		method     Method
		reasonCode ReasonCode
		degraded   bool
	}{
		{
			name: "text match folds case whitespace and canonical unicode",
			input: CompareInput{
				StudentAnswer:   "  CAFE\u0301\nAU LAIT ",
				ReferenceAnswer: "café au lait",
				Kind:            AnswerKindText,
			},
			decision: DecisionCorrect, method: MethodTextExact, reasonCode: ReasonTextExactMatch,
		},
		{
			name: "text mismatch is definitive",
			input: CompareInput{
				StudentAnswer:   "阿·贝尔",
				ReferenceAnswer: "阿*贝尔",
				Kind:            AnswerKindText,
			},
			decision: DecisionIncorrect, method: MethodTextExact, reasonCode: ReasonTextExactMismatch,
		},
		{
			name: "expression normalizes unicode operators",
			input: CompareInput{
				StudentAnswer:   "２（x − 1） × y",
				ReferenceAnswer: "２(x-1)*y",
				Kind:            AnswerKindExpression,
			},
			decision: DecisionCorrect, method: MethodExpressionExact, reasonCode: ReasonExpressionExactMatch,
		},
		{
			name: "expression mismatch needs symbolic verification",
			input: CompareInput{
				StudentAnswer:   "x+x",
				ReferenceAnswer: "2*x",
				Kind:            AnswerKindExpression,
			},
			decision: DecisionIndeterminate, method: MethodExpressionExact, reasonCode: ReasonExpressionVerificationNeeded, degraded: true,
		},
		{
			name: "proof exact match collapses whitespace",
			input: CompareInput{
				StudentAnswer:   "由连续性\n可得结论",
				ReferenceAnswer: "由连续性 可得结论",
				Kind:            AnswerKindProof,
			},
			decision: DecisionCorrect, method: MethodProofExact, reasonCode: ReasonProofExactMatch,
		},
		{
			name: "proof mismatch needs review",
			input: CompareInput{
				StudentAnswer:   "证明方法一",
				ReferenceAnswer: "证明方法二",
				Kind:            AnswerKindProof,
			},
			decision: DecisionIndeterminate, method: MethodProofExact, reasonCode: ReasonProofReviewNeeded, degraded: true,
		},
		{
			name: "empty kind remains expression compatible",
			input: CompareInput{
				StudentAnswer:   "x ≤ 1",
				ReferenceAnswer: "x<=1",
			},
			decision: DecisionCorrect, method: MethodExpressionExact, reasonCode: ReasonExpressionExactMatch,
		},
		{
			name: "kind is normalized",
			input: CompareInput{
				StudentAnswer:   "1/2",
				ReferenceAnswer: "0.5",
				Kind:            AnswerKind(" Numeric "),
			},
			decision: DecisionCorrect, method: MethodNumericExact, reasonCode: ReasonNumericExactMatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comparator.Compare(context.Background(), tt.input)
			assertResult(t, result, tt.decision, tt.method, tt.reasonCode)
			if result.Degraded != tt.degraded {
				t.Fatalf("Degraded = %v, want %v; result=%#v", result.Degraded, tt.degraded, result)
			}
			if len(result.Evidence) != 1 || result.Evidence[0].Kind == "" || result.Evidence[0].Summary == "" {
				t.Fatalf("Evidence = %#v, want one explainable item", result.Evidence)
			}
			if tt.decision == DecisionIndeterminate && result.Failure != nil {
				t.Fatalf("semantic indeterminate should not be a failure: %#v", result.Failure)
			}
		})
	}
}

func TestCompareReturnsStructuredInputFailures(t *testing.T) {
	comparator := NewComparator()
	tests := []struct {
		name       string
		input      CompareInput
		failure    FailureCode
		stage      string
		reasonCode ReasonCode
	}{
		{
			name:    "blank student",
			input:   CompareInput{StudentAnswer: " \n ", ReferenceAnswer: "x", Kind: AnswerKindExpression},
			failure: FailureInvalidInput, stage: "student_answer", reasonCode: ReasonInputInvalid,
		},
		{
			name:    "blank reference",
			input:   CompareInput{StudentAnswer: "x", ReferenceAnswer: "\t", Kind: AnswerKindExpression},
			failure: FailureInvalidInput, stage: "reference_answer", reasonCode: ReasonInputInvalid,
		},
		{
			name:    "invalid UTF8",
			input:   CompareInput{StudentAnswer: string([]byte{0xff}), ReferenceAnswer: "x", Kind: AnswerKindText},
			failure: FailureInvalidInput, stage: "student_answer", reasonCode: ReasonInputInvalid,
		},
		{
			name:    "control character",
			input:   CompareInput{StudentAnswer: "x\x00+1", ReferenceAnswer: "x+1", Kind: AnswerKindExpression},
			failure: FailureInvalidInput, stage: "student_answer", reasonCode: ReasonInputInvalid,
		},
		{
			name:    "bidi control",
			input:   CompareInput{StudentAnswer: "x\u202e+1", ReferenceAnswer: "x+1", Kind: AnswerKindExpression},
			failure: FailureInvalidInput, stage: "student_answer", reasonCode: ReasonInputInvalid,
		},
		{
			name:    "unsupported kind",
			input:   CompareInput{StudentAnswer: "x", ReferenceAnswer: "x", Kind: "matrix"},
			failure: FailureUnsupportedKind, stage: "answer_kind", reasonCode: ReasonAnswerKindUnsupported,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comparator.Compare(context.Background(), tt.input)
			assertFailure(t, result, tt.failure, tt.stage, tt.reasonCode)
		})
	}
}

func TestCompareHonorsContextAndNilContext(t *testing.T) {
	comparator := NewComparator()
	input := CompareInput{StudentAnswer: "x", ReferenceAnswer: "x", Kind: AnswerKindExpression}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	result := comparator.Compare(canceled, input)
	assertFailure(t, result, FailureCanceled, "comparison", ReasonComparisonCanceled)
	if result.Retryable || result.Failure.Retryable {
		t.Fatalf("client cancellation must not be retryable: %#v", result)
	}

	deadline, deadlineCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer deadlineCancel()
	result = comparator.Compare(deadline, input)
	assertFailure(t, result, FailureCanceled, "comparison", ReasonComparisonCanceled)
	if !result.Retryable || !result.Failure.Retryable {
		t.Fatalf("deadline failure must be retryable: %#v", result)
	}

	result = comparator.Compare(nil, input)
	assertResult(t, result, DecisionCorrect, MethodExpressionExact, ReasonExpressionExactMatch)
}

func TestCompareRejectsInvalidComparator(t *testing.T) {
	var nilComparator *Comparator
	result := nilComparator.Compare(context.Background(), CompareInput{})
	assertFailure(t, result, FailureInvalidConfig, "configuration", ReasonComparatorInvalid)

	result = (&Comparator{}).Compare(context.Background(), CompareInput{})
	assertFailure(t, result, FailureInvalidConfig, "configuration", ReasonComparatorInvalid)
}

func TestCompareEnforcesTextAndExpressionComplexityLimits(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Limits)
		answer string
	}{
		{name: "raw bytes", mutate: func(v *Limits) { v.MaxInputBytes = 4 }, answer: "12345"},
		{name: "normalized runes", mutate: func(v *Limits) { v.MaxNormalizedRunes = 3 }, answer: "abcd"},
		{name: "syntax tokens", mutate: func(v *Limits) { v.MaxSyntaxTokens = 2 }, answer: "x+x+x+x"},
		{name: "nesting depth", mutate: func(v *Limits) { v.MaxNestingDepth = 1 }, answer: "((x))"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := testLimits()
			tt.mutate(&limits)
			comparator, err := NewComparatorWithLimits(limits)
			if err != nil {
				t.Fatalf("NewComparatorWithLimits() error = %v", err)
			}
			result := comparator.Compare(context.Background(), CompareInput{
				StudentAnswer:   tt.answer,
				ReferenceAnswer: "x",
				Kind:            AnswerKindExpression,
			})
			assertFailure(t, result, FailureInputLimitExceeded, "student_answer", ReasonInputLimitExceeded)
		})
	}
}

func testLimits() Limits {
	return Limits{
		MaxInputBytes:      1024,
		MaxNormalizedRunes: 512,
		MaxSyntaxTokens:    128,
		MaxNestingDepth:    16,
		MaxNumericDigits:   128,
		MaxExponent:        100,
	}
}

func assertResult(t *testing.T, result Result, decision Decision, method Method, reasonCode ReasonCode) {
	t.Helper()
	if result.Decision != decision || result.Method != method || result.ReasonCode != reasonCode {
		t.Fatalf("result = %#v, want decision=%q method=%q reason=%q", result, decision, method, reasonCode)
	}
	if result.Reason == "" {
		t.Fatalf("Result.Reason is empty: %#v", result)
	}
	if decision != DecisionIndeterminate && result.Confidence != 1 {
		t.Fatalf("Confidence = %v, want 1 for deterministic decision", result.Confidence)
	}
}

func assertFailure(t *testing.T, result Result, code FailureCode, stage string, reasonCode ReasonCode) {
	t.Helper()
	if result.Decision != DecisionIndeterminate || result.Method != MethodNone || !result.Degraded {
		t.Fatalf("failure result shape = %#v", result)
	}
	if result.ReasonCode != reasonCode || result.Reason == "" {
		t.Fatalf("failure reason = %#v, want code %q", result, reasonCode)
	}
	if result.Failure == nil || result.Failure.Code != code || result.Failure.Stage != stage || result.Failure.Message == "" {
		t.Fatalf("Failure = %#v, want code=%q stage=%q", result.Failure, code, stage)
	}
	if result.Evidence == nil || len(result.Evidence) != 0 {
		t.Fatalf("failure Evidence = %#v, want non-nil empty slice", result.Evidence)
	}
}
