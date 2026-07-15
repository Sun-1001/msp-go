package mathsolver

// Decision is the three-state outcome of an answer comparison.
type Decision string

const (
	DecisionCorrect       Decision = "correct"
	DecisionIncorrect     Decision = "incorrect"
	DecisionIndeterminate Decision = "indeterminate"
)

// AnswerKind selects the deterministic comparison policy.
type AnswerKind string

const (
	AnswerKindNumeric    AnswerKind = "numeric"
	AnswerKindText       AnswerKind = "text"
	AnswerKindExpression AnswerKind = "expression"
	AnswerKindProof      AnswerKind = "proof"
)

// Method identifies the deterministic method that produced a result.
type Method string

const (
	MethodNone             Method = "none"
	MethodTextExact        Method = "text_exact"
	MethodExpressionExact  Method = "expression_exact"
	MethodProofExact       Method = "proof_exact"
	MethodNumericExact     Method = "numeric_exact"
	MethodNumericTolerance Method = "numeric_tolerance"
)

// ReasonCode is a stable, machine-readable explanation for a decision.
type ReasonCode string

const (
	ReasonTextExactMatch               ReasonCode = "text_exact_match"
	ReasonTextExactMismatch            ReasonCode = "text_exact_mismatch"
	ReasonExpressionExactMatch         ReasonCode = "expression_exact_match"
	ReasonExpressionVerificationNeeded ReasonCode = "expression_verification_required"
	ReasonProofExactMatch              ReasonCode = "proof_exact_match"
	ReasonProofReviewNeeded            ReasonCode = "proof_review_required"
	ReasonNumericExactMatch            ReasonCode = "numeric_exact_match"
	ReasonNumericWithinTolerance       ReasonCode = "numeric_within_tolerance"
	ReasonNumericMismatch              ReasonCode = "numeric_mismatch"
	ReasonInputInvalid                 ReasonCode = "input_invalid"
	ReasonInputLimitExceeded           ReasonCode = "input_limit_exceeded"
	ReasonNumericParseFailed           ReasonCode = "numeric_parse_failed"
	ReasonToleranceInvalid             ReasonCode = "tolerance_invalid"
	ReasonAnswerKindUnsupported        ReasonCode = "answer_kind_unsupported"
	ReasonComparisonCanceled           ReasonCode = "comparison_canceled"
	ReasonComparatorInvalid            ReasonCode = "comparator_invalid"
)

// FailureCode classifies a comparison failure without exposing implementation details.
type FailureCode string

const (
	FailureInvalidInput        FailureCode = "invalid_input"
	FailureInputLimitExceeded  FailureCode = "input_limit_exceeded"
	FailureNumericParse        FailureCode = "numeric_parse_failed"
	FailureInvalidTolerance    FailureCode = "invalid_tolerance"
	FailureUnsupportedKind     FailureCode = "unsupported_answer_kind"
	FailureCanceled            FailureCode = "canceled"
	FailureInvalidConfig       FailureCode = "invalid_configuration"
	FailureSolverUnavailable   FailureCode = "solver_unavailable"
	FailureSolverTimeout       FailureCode = "solver_timeout"
	FailureSolverInvalid       FailureCode = "solver_invalid_response"
	FailureSolverIndeterminate FailureCode = "solver_indeterminate"
	FailureVerificationFailed  FailureCode = "verification_failed"
)

// Evidence is one sanitized fact supporting a comparison result.
type Evidence struct {
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

// Failure describes why the deterministic layer could not produce a decision.
type Failure struct {
	Code      FailureCode `json:"code"`
	Stage     string      `json:"stage"`
	Message   string      `json:"message"`
	Retryable bool        `json:"retryable"`
}

// Result is an explainable, three-state answer comparison result.
type Result struct {
	Decision   Decision   `json:"decision"`
	Method     Method     `json:"method"`
	ReasonCode ReasonCode `json:"reason_code"`
	Reason     string     `json:"reason"`
	Confidence float64    `json:"confidence"`
	Degraded   bool       `json:"degraded"`
	Retryable  bool       `json:"retryable"`
	Evidence   []Evidence `json:"evidence"`
	Failure    *Failure   `json:"failure,omitempty"`
}

// Tolerance configures exact decimal absolute and relative numeric tolerances.
// Empty values mean zero tolerance.
type Tolerance struct {
	Absolute string `json:"absolute,omitempty"`
	Relative string `json:"relative,omitempty"`
}

// CompareInput contains the student and reference answers to compare.
type CompareInput struct {
	StudentAnswer   string     `json:"student_answer"`
	ReferenceAnswer string     `json:"reference_answer"`
	Kind            AnswerKind `json:"kind"`
	Tolerance       Tolerance  `json:"tolerance"`
}

// Limits bounds normalization and exact-number construction costs.
type Limits struct {
	MaxInputBytes      int
	MaxNormalizedRunes int
	MaxSyntaxTokens    int
	MaxNestingDepth    int
	MaxNumericDigits   int
	MaxExponent        int
}

// Comparator performs bounded deterministic answer comparisons.
type Comparator struct {
	limits Limits
}
