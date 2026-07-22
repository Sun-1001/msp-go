package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	adminaiconfigapp "mathstudy/backend-go/internal/application/adminaiconfig"
	airiskapp "mathstudy/backend-go/internal/application/airisk"
	"mathstudy/backend-go/internal/platform/outbound"
)

const (
	contentModeratorAgentType = "content_moderator"
	maxModerationInputRunes   = 12_000
	maxModerationResponseSize = 1 << 20
)

// HTTPDoer is the HTTP boundary used by the moderation adapter.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// RuntimeConfigProvider loads the encrypted-at-rest provider configuration.
type RuntimeConfigProvider interface {
	RuntimeConfigs(context.Context, string) ([]adminaiconfigapp.RuntimeConfig, bool, error)
}

// Reviewer calls an OpenAI-compatible moderations endpoint.
type Reviewer struct {
	provider   RuntimeConfigProvider
	httpClient HTTPDoer
	retryDelay func(int) time.Duration
}

// NewReviewer creates a provider-backed content reviewer.
func NewReviewer(provider RuntimeConfigProvider, clients ...HTTPDoer) (*Reviewer, error) {
	if provider == nil {
		return nil, errors.New("moderation runtime config provider is nil")
	}
	if len(clients) > 1 {
		return nil, errors.New("moderation reviewer accepts at most one HTTP client")
	}
	var client HTTPDoer
	if len(clients) == 1 {
		client = clients[0]
	}
	return &Reviewer{
		provider:   provider,
		httpClient: client,
		retryDelay: func(attempt int) time.Duration { return time.Duration(attempt+1) * 100 * time.Millisecond },
	}, nil
}

// Review evaluates one student input and returns normalized category scores.
func (r *Reviewer) Review(ctx context.Context, content string) (airiskapp.ModelReviewResult, error) {
	if r == nil || r.provider == nil {
		return airiskapp.ModelReviewResult{}, errors.New("moderation reviewer is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return airiskapp.ModelReviewResult{}, err
	}
	content = trimModerationInput(content)
	if content == "" {
		return airiskapp.ModelReviewResult{}, errors.New("moderation input is empty")
	}
	runtimes, ok, err := r.provider.RuntimeConfigs(ctx, contentModeratorAgentType)
	if err != nil {
		return airiskapp.ModelReviewResult{}, fmt.Errorf("load moderation runtime config: %w", err)
	}
	if !ok || len(runtimes) == 0 {
		return airiskapp.ModelReviewResult{}, errors.New("content moderator agent is not configured")
	}
	var lastErr error
	for attempt, runtime := range runtimes {
		if err := ctx.Err(); err != nil {
			return airiskapp.ModelReviewResult{}, err
		}
		baseURL, err := validateRuntimeConfig(runtime)
		if err != nil {
			lastErr = err
			if attempt == len(runtimes)-1 {
				break
			}
			continue
		}
		result, err := r.reviewOnce(ctx, runtime, baseURL, content)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if ctxErr := ctx.Err(); ctxErr != nil {
			return airiskapp.ModelReviewResult{}, ctxErr
		}
		if !isRetryable(err) || attempt == len(runtimes)-1 {
			break
		}
		if err := waitForRetry(ctx, r.retryDelay(attempt)); err != nil {
			return airiskapp.ModelReviewResult{}, err
		}
	}
	return airiskapp.ModelReviewResult{}, fmt.Errorf("moderation request failed: %w", lastErr)
}

func (r *Reviewer) reviewOnce(
	ctx context.Context,
	runtime adminaiconfigapp.RuntimeConfig,
	baseURL string,
	content string,
) (airiskapp.ModelReviewResult, error) {
	payload, err := json.Marshal(struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}{Model: strings.TrimSpace(runtime.Model), Input: content})
	if err != nil {
		return airiskapp.ModelReviewResult{}, fmt.Errorf("encode moderation request: %w", err)
	}
	requestContext, cancel := context.WithTimeout(ctx, runtime.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(
		requestContext,
		http.MethodPost,
		joinProviderURL(baseURL, "/v1/moderations"),
		bytes.NewReader(payload),
	)
	if err != nil {
		return airiskapp.ModelReviewResult{}, fmt.Errorf("create moderation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(runtime.APIKey))
	req.Header.Set("Content-Type", "application/json")
	client := r.httpClient
	if client == nil {
		client = outbound.NewPublicHTTPSClient(runtime.Timeout)
	}
	resp, err := client.Do(req)
	if err != nil {
		return airiskapp.ModelReviewResult{}, requestError{cause: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return airiskapp.ModelReviewResult{}, requestError{status: resp.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxModerationResponseSize+1))
	if err != nil {
		return airiskapp.ModelReviewResult{}, fmt.Errorf("read moderation response: %w", err)
	}
	if len(raw) > maxModerationResponseSize {
		return airiskapp.ModelReviewResult{}, errors.New("moderation response is too large")
	}
	var response struct {
		Results []struct {
			CategoryScores map[string]float64 `json:"category_scores"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return airiskapp.ModelReviewResult{}, errors.New("moderation response is invalid JSON")
	}
	if len(response.Results) != 1 {
		return airiskapp.ModelReviewResult{}, errors.New("moderation response must contain exactly one result")
	}
	scores := response.Results[0].CategoryScores
	if len(scores) == 0 {
		return airiskapp.ModelReviewResult{}, errors.New("moderation response has no category scores")
	}
	for category, score := range scores {
		if strings.TrimSpace(category) == "" || math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 1 {
			return airiskapp.ModelReviewResult{}, errors.New("moderation response has invalid category scores")
		}
	}
	return airiskapp.ModelReviewResult{
		Model:          strings.TrimSpace(runtime.Model),
		CategoryScores: cloneScores(scores),
	}, nil
}

func validateRuntimeConfig(runtime adminaiconfigapp.RuntimeConfig) (string, error) {
	baseURL, err := outbound.NormalizePublicHTTPSBaseURL(runtime.BaseURL)
	if err != nil {
		return "", fmt.Errorf("moderation base URL is invalid: %w", err)
	}
	if strings.TrimSpace(runtime.APIKey) == "" {
		return "", errors.New("moderation API key is unavailable")
	}
	if strings.TrimSpace(runtime.Model) == "" {
		return "", errors.New("moderation model is unavailable")
	}
	if runtime.Timeout <= 0 {
		return "", errors.New("moderation timeout must be greater than zero")
	}
	return baseURL, nil
}

func trimModerationInput(content string) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= maxModerationInputRunes {
		return content
	}
	const marker = "\n[...truncated...]\n"
	markerRunes := []rune(marker)
	remaining := maxModerationInputRunes - len(markerRunes)
	left := remaining / 2
	right := remaining - left
	return string(runes[:left]) + marker + string(runes[len(runes)-right:])
}

func joinProviderURL(baseURL string, apiPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(apiPath, "/v1/") {
		return base + strings.TrimPrefix(apiPath, "/v1")
	}
	return base + apiPath
}

func cloneScores(scores map[string]float64) map[string]float64 {
	cloned := make(map[string]float64, len(scores))
	for category, score := range scores {
		cloned[category] = score
	}
	return cloned
}

type requestError struct {
	status int
	cause  error
}

func (e requestError) Error() string {
	if e.status > 0 {
		return fmt.Sprintf("moderation API returned HTTP %d", e.status)
	}
	return "moderation API request failed"
}

func (e requestError) Unwrap() error { return e.cause }

func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var requestErr requestError
	if !errors.As(err, &requestErr) {
		return false
	}
	status := requestErr.status
	return status == 0 || status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound || status == http.StatusRequestTimeout || status == http.StatusConflict || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
