package answerocr

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	// MaxImageSize matches the authenticated image upload boundary.
	MaxImageSize = 10 * 1024 * 1024
	// MaxAnswerLength bounds model output before it reaches grading or persistence.
	MaxAnswerLength = 16 * 1024
	// MinimumConfidence keeps low-confidence recognition from affecting learning state.
	MinimumConfidence = 0.6
)

var (
	// ErrInvalidImage indicates an invalid, missing, or unsupported image reference.
	ErrInvalidImage = errors.New("invalid answer image")
	// ErrUnreadable indicates that no sufficiently reliable answer could be extracted.
	ErrUnreadable = errors.New("answer image is unreadable")
	// ErrUnavailable indicates that the image or recognition backend is unavailable.
	ErrUnavailable = errors.New("answer OCR is unavailable")
	// ErrTimeout indicates that image loading or recognition exceeded its deadline.
	ErrTimeout = errors.New("answer OCR timed out")
)

// Image contains validated image bytes supplied to a recognizer.
type Image struct {
	Data     []byte
	MIMEType string
}

// ImageLoader resolves a trusted upload reference into validated image bytes.
type ImageLoader interface {
	LoadImage(context.Context, string) (Image, error)
}

// RecognizeInput carries image content and answer-shape context to a recognizer.
type RecognizeInput struct {
	Image      Image
	AnswerType string
}

// Result is the strict structured output produced by an answer recognizer.
type Result struct {
	Status      string
	AnswerLatex string
	Confidence  float64
	Reason      string
}

// Recognizer extracts a final answer from validated image bytes.
type Recognizer interface {
	Recognize(context.Context, RecognizeInput) (Result, error)
}

// Service resolves and recognizes image answers before any learning-state transaction.
type Service struct {
	loader     ImageLoader
	recognizer Recognizer
}

// NewService creates an answer OCR application service.
func NewService(loader ImageLoader, recognizer Recognizer) (*Service, error) {
	if loader == nil {
		return nil, errors.New("answer OCR image loader is nil")
	}
	if recognizer == nil {
		return nil, errors.New("answer OCR recognizer is nil")
	}
	return &Service{loader: loader, recognizer: recognizer}, nil
}

// Recognize loads an image reference and returns only a validated, reliable answer.
func (s *Service) Recognize(ctx context.Context, imageReference string, answerType string) (Result, error) {
	if s == nil || s.loader == nil || s.recognizer == nil {
		return Result{}, ErrUnavailable
	}
	imageReference = strings.TrimSpace(imageReference)
	if imageReference == "" {
		return Result{}, ErrInvalidImage
	}

	image, err := s.loader.LoadImage(ctx, imageReference)
	if err != nil {
		return Result{}, normalizeBoundaryError(err, "load answer image")
	}
	if err := validateImage(image); err != nil {
		return Result{}, err
	}

	result, err := s.recognizer.Recognize(ctx, RecognizeInput{
		Image:      image,
		AnswerType: strings.TrimSpace(answerType),
	})
	if err != nil {
		return Result{}, normalizeBoundaryError(err, "recognize answer image")
	}
	return normalizeResult(result)
}

func validateImage(image Image) error {
	if len(image.Data) == 0 || len(image.Data) > MaxImageSize {
		return ErrInvalidImage
	}
	switch strings.ToLower(strings.TrimSpace(image.MIMEType)) {
	case "image/gif", "image/jpeg", "image/png":
		return nil
	default:
		return ErrInvalidImage
	}
}

func normalizeResult(result Result) (Result, error) {
	result.Status = strings.ToLower(strings.TrimSpace(result.Status))
	result.AnswerLatex = strings.TrimSpace(result.AnswerLatex)
	result.Reason = strings.TrimSpace(result.Reason)
	if math.IsNaN(result.Confidence) || math.IsInf(result.Confidence, 0) || result.Confidence < 0 || result.Confidence > 1 {
		return Result{}, ErrUnavailable
	}

	switch result.Status {
	case "unreadable":
		return Result{}, ErrUnreadable
	case "ok":
		if result.AnswerLatex == "" || len(result.AnswerLatex) > MaxAnswerLength {
			return Result{}, ErrUnavailable
		}
		if result.Confidence < MinimumConfidence {
			return Result{}, ErrUnreadable
		}
		return result, nil
	default:
		return Result{}, ErrUnavailable
	}
}

func normalizeBoundaryError(err error, operation string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrTimeout):
		return fmt.Errorf("%w: %s", ErrTimeout, operation)
	case errors.Is(err, ErrInvalidImage):
		return fmt.Errorf("%w: %s", ErrInvalidImage, operation)
	case errors.Is(err, ErrUnreadable):
		return fmt.Errorf("%w: %s", ErrUnreadable, operation)
	case errors.Is(err, ErrUnavailable):
		return fmt.Errorf("%w: %s", ErrUnavailable, operation)
	default:
		return fmt.Errorf("%w: %s", ErrUnavailable, operation)
	}
}
