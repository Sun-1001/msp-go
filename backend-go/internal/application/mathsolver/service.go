package mathsolver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	defaultMaxInputBytes      = 16 * 1024
	defaultMaxNormalizedRunes = 4096
	defaultMaxSyntaxTokens    = 1024
	defaultMaxNestingDepth    = 64
	defaultMaxNumericDigits   = 1024
	defaultMaxExponent        = 10_000
)

var mathSymbolReplacer = strings.NewReplacer(
	"−", "-",
	"﹣", "-",
	"－", "-",
	"＋", "+",
	"×", "*",
	"∙", "*",
	"⋅", "*",
	"·", "*",
	"＊", "*",
	"÷", "/",
	"∕", "/",
	"⁄", "/",
	"／", "/",
	"＾", "^",
	"＝", "=",
	"≤", "<=",
	"≥", ">=",
	"≠", "!=",
	"＜", "<",
	"＞", ">",
	"（", "(",
	"）", ")",
	"［", "[",
	"］", "]",
	"｛", "{",
	"｝", "}",
	"，", ",",
	"．", ".",
)

type inputIssue struct {
	reasonCode  ReasonCode
	failureCode FailureCode
	message     string
}

func (e inputIssue) Error() string {
	return e.message
}

// NewComparator creates a comparator with conservative resource limits.
func NewComparator() *Comparator {
	return &Comparator{limits: Limits{
		MaxInputBytes:      defaultMaxInputBytes,
		MaxNormalizedRunes: defaultMaxNormalizedRunes,
		MaxSyntaxTokens:    defaultMaxSyntaxTokens,
		MaxNestingDepth:    defaultMaxNestingDepth,
		MaxNumericDigits:   defaultMaxNumericDigits,
		MaxExponent:        defaultMaxExponent,
	}}
}

// NewComparatorWithLimits creates a comparator with explicit positive limits.
func NewComparatorWithLimits(limits Limits) (*Comparator, error) {
	checks := []struct {
		name  string
		value int
	}{
		{name: "MaxInputBytes", value: limits.MaxInputBytes},
		{name: "MaxNormalizedRunes", value: limits.MaxNormalizedRunes},
		{name: "MaxSyntaxTokens", value: limits.MaxSyntaxTokens},
		{name: "MaxNestingDepth", value: limits.MaxNestingDepth},
		{name: "MaxNumericDigits", value: limits.MaxNumericDigits},
		{name: "MaxExponent", value: limits.MaxExponent},
	}
	for _, check := range checks {
		if check.value <= 0 {
			return nil, fmt.Errorf("math solver limit %s must be greater than zero", check.name)
		}
	}
	return &Comparator{limits: limits}, nil
}

// Compare performs a bounded deterministic comparison without collapsing uncertainty into incorrectness.
func (c *Comparator) Compare(ctx context.Context, input CompareInput) Result {
	if c == nil || !validLimits(c.limits) {
		return failureResult(
			ReasonComparatorInvalid,
			"确定性比较器配置无效",
			FailureInvalidConfig,
			"configuration",
			false,
		)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return contextFailure(err)
	}

	kind := AnswerKind(strings.ToLower(strings.TrimSpace(string(input.Kind))))
	if kind == "" {
		kind = AnswerKindExpression
	}
	if !supportedAnswerKind(kind) {
		return failureResult(
			ReasonAnswerKindUnsupported,
			"当前确定性比较器不支持该答案类型",
			FailureUnsupportedKind,
			"answer_kind",
			false,
		)
	}

	student, err := c.normalize(input.StudentAnswer, kind)
	if err != nil {
		return inputFailure(err, "student_answer")
	}
	if err := ctx.Err(); err != nil {
		return contextFailure(err)
	}
	reference, err := c.normalize(input.ReferenceAnswer, kind)
	if err != nil {
		return inputFailure(err, "reference_answer")
	}

	switch kind {
	case AnswerKindNumeric:
		return c.compareNumeric(student, reference, input.Tolerance)
	case AnswerKindText:
		return compareExactText(student, reference)
	case AnswerKindExpression:
		return compareExactExpression(student, reference)
	case AnswerKindProof:
		return compareExactProof(student, reference)
	default:
		return failureResult(
			ReasonAnswerKindUnsupported,
			"当前确定性比较器不支持该答案类型",
			FailureUnsupportedKind,
			"answer_kind",
			false,
		)
	}
}

func validLimits(limits Limits) bool {
	return limits.MaxInputBytes > 0 &&
		limits.MaxNormalizedRunes > 0 &&
		limits.MaxSyntaxTokens > 0 &&
		limits.MaxNestingDepth > 0 &&
		limits.MaxNumericDigits > 0 &&
		limits.MaxExponent > 0
}

func supportedAnswerKind(kind AnswerKind) bool {
	switch kind {
	case AnswerKindNumeric, AnswerKindText, AnswerKindExpression, AnswerKindProof:
		return true
	default:
		return false
	}
}

func (c *Comparator) normalize(value string, kind AnswerKind) (string, error) {
	if len(value) > c.limits.MaxInputBytes {
		return "", inputIssue{
			reasonCode:  ReasonInputLimitExceeded,
			failureCode: FailureInputLimitExceeded,
			message:     "答案长度超过确定性比较上限",
		}
	}
	if !utf8.ValidString(value) {
		return "", inputIssue{
			reasonCode:  ReasonInputInvalid,
			failureCode: FailureInvalidInput,
			message:     "答案不是有效的 UTF-8 文本",
		}
	}

	value = norm.NFC.String(value)
	if kind != AnswerKindText {
		value = mathSymbolReplacer.Replace(value)
	}
	compactWhitespace := kind == AnswerKindNumeric || kind == AnswerKindExpression
	var builder strings.Builder
	builder.Grow(len(value))
	runeCount := 0
	syntaxTokens := 0
	depth := 0
	spacePending := false
	for _, current := range value {
		if unicode.IsSpace(current) {
			if !compactWhitespace && builder.Len() > 0 {
				spacePending = true
			}
			continue
		}
		if unsafeControlRune(current) {
			return "", inputIssue{
				reasonCode:  ReasonInputInvalid,
				failureCode: FailureInvalidInput,
				message:     "答案包含不允许的控制字符",
			}
		}
		if spacePending {
			builder.WriteByte(' ')
			runeCount++
			spacePending = false
		}
		builder.WriteRune(current)
		runeCount++
		if runeCount > c.limits.MaxNormalizedRunes {
			return "", inputIssue{
				reasonCode:  ReasonInputLimitExceeded,
				failureCode: FailureInputLimitExceeded,
				message:     "规范化答案长度超过确定性比较上限",
			}
		}
		if isSyntaxToken(current) {
			syntaxTokens++
			if syntaxTokens > c.limits.MaxSyntaxTokens {
				return "", inputIssue{
					reasonCode:  ReasonInputLimitExceeded,
					failureCode: FailureInputLimitExceeded,
					message:     "答案语法复杂度超过确定性比较上限",
				}
			}
		}
		switch current {
		case '(', '[', '{':
			depth++
			if depth > c.limits.MaxNestingDepth {
				return "", inputIssue{
					reasonCode:  ReasonInputLimitExceeded,
					failureCode: FailureInputLimitExceeded,
					message:     "答案嵌套深度超过确定性比较上限",
				}
			}
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		}
	}
	normalized := strings.TrimSpace(builder.String())
	if normalized == "" {
		return "", inputIssue{
			reasonCode:  ReasonInputInvalid,
			failureCode: FailureInvalidInput,
			message:     "答案不能为空",
		}
	}
	return normalized, nil
}

func unsafeControlRune(value rune) bool {
	if unicode.IsControl(value) {
		return true
	}
	switch {
	case value >= '\u200b' && value <= '\u200f':
		return true
	case value >= '\u202a' && value <= '\u202e':
		return true
	case value >= '\u2060' && value <= '\u206f':
		return true
	case value == '\ufeff':
		return true
	default:
		return false
	}
}

func isSyntaxToken(value rune) bool {
	return strings.ContainsRune("+-*/^=<>!&|%,;:\\", value)
}

func compareExactText(student string, reference string) Result {
	if strings.EqualFold(student, reference) {
		return decisiveResult(
			DecisionCorrect,
			MethodTextExact,
			ReasonTextExactMatch,
			"规范化文本与标准答案一致",
			Evidence{Kind: "normalized_text", Summary: "规范化文本完全一致"},
		)
	}
	return decisiveResult(
		DecisionIncorrect,
		MethodTextExact,
		ReasonTextExactMismatch,
		"规范化文本与标准答案不一致",
		Evidence{Kind: "normalized_text", Summary: "规范化文本存在差异"},
	)
}

func compareExactExpression(student string, reference string) Result {
	if student == reference {
		return decisiveResult(
			DecisionCorrect,
			MethodExpressionExact,
			ReasonExpressionExactMatch,
			"规范化表达式与标准答案一致",
			Evidence{Kind: "normalized_expression", Summary: "规范化表达式完全一致"},
		)
	}
	return indeterminateResult(
		MethodExpressionExact,
		ReasonExpressionVerificationNeeded,
		"表达式形式不同，需要符号求解器验证数学等价性",
		Evidence{Kind: "comparison_boundary", Summary: "字符串差异不能证明表达式不等价"},
	)
}

func compareExactProof(student string, reference string) Result {
	if student == reference {
		return decisiveResult(
			DecisionCorrect,
			MethodProofExact,
			ReasonProofExactMatch,
			"规范化证明文本与参考答案一致",
			Evidence{Kind: "normalized_proof", Summary: "规范化证明文本完全一致"},
		)
	}
	return indeterminateResult(
		MethodProofExact,
		ReasonProofReviewNeeded,
		"证明文本不同，需要语义验证或人工复核",
		Evidence{Kind: "comparison_boundary", Summary: "字符串差异不能证明推理错误"},
	)
}

func decisiveResult(decision Decision, method Method, reasonCode ReasonCode, reason string, evidence Evidence) Result {
	return Result{
		Decision:   decision,
		Method:     method,
		ReasonCode: reasonCode,
		Reason:     reason,
		Confidence: 1,
		Evidence:   []Evidence{evidence},
	}
}

func indeterminateResult(method Method, reasonCode ReasonCode, reason string, evidence Evidence) Result {
	return Result{
		Decision:   DecisionIndeterminate,
		Method:     method,
		ReasonCode: reasonCode,
		Reason:     reason,
		Degraded:   true,
		Evidence:   []Evidence{evidence},
	}
}

func inputFailure(err error, stage string) Result {
	var issue inputIssue
	if errors.As(err, &issue) {
		return failureResult(issue.reasonCode, issue.message, issue.failureCode, stage, false)
	}
	return failureResult(ReasonInputInvalid, "答案输入无效", FailureInvalidInput, stage, false)
}

func failureResult(reasonCode ReasonCode, reason string, code FailureCode, stage string, retryable bool) Result {
	return Result{
		Decision:   DecisionIndeterminate,
		Method:     MethodNone,
		ReasonCode: reasonCode,
		Reason:     reason,
		Degraded:   true,
		Retryable:  retryable,
		Evidence:   []Evidence{},
		Failure: &Failure{
			Code:      code,
			Stage:     stage,
			Message:   reason,
			Retryable: retryable,
		},
	}
}

func contextFailure(err error) Result {
	retryable := errors.Is(err, context.DeadlineExceeded)
	reason := "答案比较已取消"
	if retryable {
		reason = "答案比较超时"
	}
	return failureResult(ReasonComparisonCanceled, reason, FailureCanceled, "comparison", retryable)
}
