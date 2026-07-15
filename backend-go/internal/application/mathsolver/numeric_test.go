package mathsolver

import (
	"context"
	"strings"
	"testing"
)

func TestCompareNumericRecognizesExactEquivalentForms(t *testing.T) {
	comparator := NewComparator()
	tests := []struct {
		name      string
		student   string
		reference string
	}{
		{name: "integer and decimal", student: "1", reference: "1.0"},
		{name: "fraction and decimal", student: "1/2", reference: "0.5"},
		{name: "decimal fraction", student: "1.5/3", reference: ".5"},
		{name: "negative denominator", student: "1/-2", reference: "-.5"},
		{name: "leading zeros and plus", student: "+001.2500", reference: "5/4"},
		{name: "scientific positive", student: "1e3", reference: "1000"},
		{name: "scientific explicit plus", student: "1E+3", reference: "1000."},
		{name: "scientific negative", student: "2E-3", reference: "0.002"},
		{name: "unicode multiplication science", student: "2 × 10＾3", reference: "2000"},
		{name: "unicode minus", student: "−3", reference: "-3"},
		{name: "unicode fraction slash", student: "1⁄4", reference: ".25"},
		{name: "negative zero", student: "-0e100", reference: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comparator.Compare(context.Background(), CompareInput{
				StudentAnswer:   tt.student,
				ReferenceAnswer: tt.reference,
				Kind:            AnswerKindNumeric,
			})
			assertResult(t, result, DecisionCorrect, MethodNumericExact, ReasonNumericExactMatch)
			if result.Degraded || result.Retryable || result.Failure != nil {
				t.Fatalf("exact numeric result = %#v, want non-degraded success", result)
			}
		})
	}
}

func TestCompareNumericAppliesAbsoluteAndRelativeTolerance(t *testing.T) {
	comparator := NewComparator()
	tests := []struct {
		name       string
		student    string
		reference  string
		tolerance  Tolerance
		decision   Decision
		method     Method
		reasonCode ReasonCode
	}{
		{
			name: "exact remains exact when tolerance configured", student: "1", reference: "1.0",
			tolerance: Tolerance{Absolute: "0.1", Relative: "0.1"},
			decision:  DecisionCorrect, method: MethodNumericExact, reasonCode: ReasonNumericExactMatch,
		},
		{
			name: "absolute boundary inclusive", student: "1.01", reference: "1",
			tolerance: Tolerance{Absolute: "0.01"},
			decision:  DecisionCorrect, method: MethodNumericTolerance, reasonCode: ReasonNumericWithinTolerance,
		},
		{
			name: "absolute outside", student: "1.011", reference: "1",
			tolerance: Tolerance{Absolute: "0.01"},
			decision:  DecisionIncorrect, method: MethodNumericTolerance, reasonCode: ReasonNumericMismatch,
		},
		{
			name: "relative boundary inclusive", student: "101", reference: "100",
			tolerance: Tolerance{Relative: "1e-2"},
			decision:  DecisionCorrect, method: MethodNumericTolerance, reasonCode: ReasonNumericWithinTolerance,
		},
		{
			name: "relative uses absolute reference magnitude", student: "-101", reference: "-100",
			tolerance: Tolerance{Relative: "0.01"},
			decision:  DecisionCorrect, method: MethodNumericTolerance, reasonCode: ReasonNumericWithinTolerance,
		},
		{
			name: "larger absolute threshold wins", student: "0.5", reference: "0",
			tolerance: Tolerance{Absolute: "1/2", Relative: "1"},
			decision:  DecisionCorrect, method: MethodNumericTolerance, reasonCode: ReasonNumericWithinTolerance,
		},
		{
			name: "relative tolerance cannot move zero", student: "0.0001", reference: "0",
			tolerance: Tolerance{Relative: "1"},
			decision:  DecisionIncorrect, method: MethodNumericTolerance, reasonCode: ReasonNumericMismatch,
		},
		{
			name: "zero tolerance is exact mismatch", student: "2", reference: "3",
			tolerance: Tolerance{Absolute: "0", Relative: "0"},
			decision:  DecisionIncorrect, method: MethodNumericExact, reasonCode: ReasonNumericMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comparator.Compare(context.Background(), CompareInput{
				StudentAnswer:   tt.student,
				ReferenceAnswer: tt.reference,
				Kind:            AnswerKindNumeric,
				Tolerance:       tt.tolerance,
			})
			assertResult(t, result, tt.decision, tt.method, tt.reasonCode)
			if len(result.Evidence) != 1 || result.Evidence[0].Kind == "" {
				t.Fatalf("Evidence = %#v, want numeric evidence", result.Evidence)
			}
		})
	}
}

func TestCompareNumericRejectsMalformedValues(t *testing.T) {
	comparator := NewComparator()
	tests := []struct {
		name      string
		student   string
		reference string
		stage     string
	}{
		{name: "letters", student: "abc", reference: "1", stage: "student_numeric"},
		{name: "NaN", student: "NaN", reference: "1", stage: "student_numeric"},
		{name: "infinity", student: "Inf", reference: "1", stage: "student_numeric"},
		{name: "hexadecimal", student: "0x10", reference: "16", stage: "student_numeric"},
		{name: "zero denominator", student: "1/0", reference: "1", stage: "student_numeric"},
		{name: "multiple fractions", student: "1/2/3", reference: "1", stage: "student_numeric"},
		{name: "missing exponent", student: "1e", reference: "1", stage: "student_numeric"},
		{name: "multiple exponents", student: "1e2e3", reference: "1", stage: "student_numeric"},
		{name: "bad ten-power form", student: "1*10^", reference: "1", stage: "student_numeric"},
		{name: "expression is not scalar", student: "1+1", reference: "2", stage: "student_numeric"},
		{name: "invalid reference", student: "1", reference: "pi", stage: "reference_numeric"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comparator.Compare(context.Background(), CompareInput{
				StudentAnswer:   tt.student,
				ReferenceAnswer: tt.reference,
				Kind:            AnswerKindNumeric,
			})
			assertFailure(t, result, FailureNumericParse, tt.stage, ReasonNumericParseFailed)
			if result.Retryable {
				t.Fatalf("malformed numeric value must not be retryable: %#v", result)
			}
		})
	}
}

func TestCompareNumericRejectsInvalidTolerances(t *testing.T) {
	comparator := NewComparator()
	tests := []struct {
		name      string
		tolerance Tolerance
		stage     string
	}{
		{name: "negative absolute", tolerance: Tolerance{Absolute: "-0.1"}, stage: "absolute_tolerance"},
		{name: "malformed absolute", tolerance: Tolerance{Absolute: "close"}, stage: "absolute_tolerance"},
		{name: "zero denominator absolute", tolerance: Tolerance{Absolute: "1/0"}, stage: "absolute_tolerance"},
		{name: "negative relative", tolerance: Tolerance{Relative: "-1e-2"}, stage: "relative_tolerance"},
		{name: "malformed relative", tolerance: Tolerance{Relative: "1%"}, stage: "relative_tolerance"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comparator.Compare(context.Background(), CompareInput{
				StudentAnswer: "1", ReferenceAnswer: "1.1", Kind: AnswerKindNumeric, Tolerance: tt.tolerance,
			})
			assertFailure(t, result, FailureInvalidTolerance, tt.stage, ReasonToleranceInvalid)
		})
	}
}

func TestCompareNumericEnforcesDigitAndExponentLimits(t *testing.T) {
	limits := testLimits()
	limits.MaxNumericDigits = 4
	limits.MaxExponent = 5
	comparator, err := NewComparatorWithLimits(limits)
	if err != nil {
		t.Fatalf("NewComparatorWithLimits() error = %v", err)
	}

	tests := []struct {
		name    string
		student string
		stage   string
	}{
		{name: "too many digits", student: "12345", stage: "student_numeric"},
		{name: "positive exponent over limit", student: "1e6", stage: "student_numeric"},
		{name: "negative exponent over limit", student: "1e-6", stage: "student_numeric"},
		{name: "huge exponent fails before integer overflow", student: "1e99999999999999999999", stage: "student_numeric"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := comparator.Compare(context.Background(), CompareInput{
				StudentAnswer: tt.student, ReferenceAnswer: "1", Kind: AnswerKindNumeric,
			})
			assertFailure(t, result, FailureInputLimitExceeded, tt.stage, ReasonInputLimitExceeded)
		})
	}

	result := comparator.Compare(context.Background(), CompareInput{
		StudentAnswer: "1e5", ReferenceAnswer: "1*10^5", Kind: AnswerKindNumeric,
	})
	assertResult(t, result, DecisionCorrect, MethodNumericExact, ReasonNumericExactMatch)
}

func TestCompareNumericEnforcesToleranceLimits(t *testing.T) {
	limits := testLimits()
	limits.MaxNumericDigits = 4
	limits.MaxExponent = 5
	comparator, err := NewComparatorWithLimits(limits)
	if err != nil {
		t.Fatalf("NewComparatorWithLimits() error = %v", err)
	}

	result := comparator.Compare(context.Background(), CompareInput{
		StudentAnswer: "1", ReferenceAnswer: "2", Kind: AnswerKindNumeric,
		Tolerance: Tolerance{Absolute: "0.00001"},
	})
	assertFailure(t, result, FailureInputLimitExceeded, "absolute_tolerance", ReasonInputLimitExceeded)

	result = comparator.Compare(context.Background(), CompareInput{
		StudentAnswer: "1", ReferenceAnswer: "2", Kind: AnswerKindNumeric,
		Tolerance: Tolerance{Relative: "1e6"},
	})
	assertFailure(t, result, FailureInputLimitExceeded, "relative_tolerance", ReasonInputLimitExceeded)
}

func TestParseBoundedExponentBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		maximum int
		want    int
		failure FailureCode
	}{
		{name: "positive boundary", value: "+5", maximum: 5, want: 5},
		{name: "negative boundary", value: "-5", maximum: 5, want: -5},
		{name: "leading zeros", value: "0005", maximum: 5, want: 5},
		{name: "over small maximum", value: "6", maximum: 5, failure: FailureInputLimitExceeded},
		{name: "empty", value: "", maximum: 5, failure: FailureNumericParse},
		{name: "sign only", value: "+", maximum: 5, failure: FailureNumericParse},
		{name: "not digits", value: "2.0", maximum: 5, failure: FailureNumericParse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBoundedExponent(tt.value, tt.maximum)
			if tt.failure == "" {
				if err != nil || got != tt.want {
					t.Fatalf("parseBoundedExponent(%q) = %d, %v, want %d", tt.value, got, err, tt.want)
				}
				return
			}
			var issue inputIssue
			if err == nil || !strings.Contains(err.Error(), "数值") && !strings.Contains(err.Error(), "指数") {
				t.Fatalf("parseBoundedExponent(%q) error = %v", tt.value, err)
			}
			if !asInputIssue(err, &issue) || issue.failureCode != tt.failure {
				t.Fatalf("issue = %#v, want failure %q", issue, tt.failure)
			}
		})
	}
}

func asInputIssue(err error, target *inputIssue) bool {
	issue, ok := err.(inputIssue)
	if !ok {
		return false
	}
	*target = issue
	return true
}
