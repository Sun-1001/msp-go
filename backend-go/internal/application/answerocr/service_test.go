package answerocr

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestNewServiceRejectsMissingDependencies(t *testing.T) {
	recognizer := &fakeRecognizer{}
	loader := &fakeLoader{}
	if _, err := NewService(nil, recognizer); err == nil {
		t.Fatal("NewService(nil, recognizer) error = nil")
	}
	if _, err := NewService(loader, nil); err == nil {
		t.Fatal("NewService(loader, nil) error = nil")
	}
}

func TestServiceRecognizeReturnsNormalizedReliableAnswer(t *testing.T) {
	loader := &fakeLoader{image: Image{Data: []byte("png"), MIMEType: " image/png "}}
	recognizer := &fakeRecognizer{result: Result{
		Status:      " OK ",
		AnswerLatex: "  x + 1  ",
		Confidence:  0.92,
		Reason:      " clear final line ",
	}}
	service, err := NewService(loader, recognizer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.Recognize(context.Background(), " image-ref ", " expression ")
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if loader.reference != "image-ref" || recognizer.input.AnswerType != "expression" {
		t.Fatalf("loader reference=%q recognizer input=%#v", loader.reference, recognizer.input)
	}
	if result.Status != "ok" || result.AnswerLatex != "x + 1" || result.Reason != "clear final line" || result.Confidence != 0.92 {
		t.Fatalf("result = %#v", result)
	}
}

func TestServiceRecognizeRejectsInvalidImageBeforeRecognizer(t *testing.T) {
	tests := []struct {
		name  string
		image Image
	}{
		{name: "empty", image: Image{MIMEType: "image/png"}},
		{name: "oversized", image: Image{Data: make([]byte, MaxImageSize+1), MIMEType: "image/png"}},
		{name: "unsupported MIME", image: Image{Data: []byte("svg"), MIMEType: "image/svg+xml"}},
		{name: "WebP without decoder", image: Image{Data: []byte("RIFF....WEBP"), MIMEType: "image/webp"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recognizer := &fakeRecognizer{}
			service, err := NewService(&fakeLoader{image: tt.image}, recognizer)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			_, err = service.Recognize(context.Background(), "image-ref", "expression")
			if !errors.Is(err, ErrInvalidImage) || recognizer.called {
				t.Fatalf("Recognize() error=%v recognizer called=%t", err, recognizer.called)
			}
		})
	}
}

func TestServiceRecognizeMapsBoundaryFailures(t *testing.T) {
	tests := []struct {
		name       string
		loaderErr  error
		recognizer error
		want       error
	}{
		{name: "invalid reference", loaderErr: ErrInvalidImage, want: ErrInvalidImage},
		{name: "loader timeout", loaderErr: context.DeadlineExceeded, want: ErrTimeout},
		{name: "loader unavailable", loaderErr: errors.New("storage down"), want: ErrUnavailable},
		{name: "recognizer timeout", recognizer: context.DeadlineExceeded, want: ErrTimeout},
		{name: "recognizer unreadable", recognizer: ErrUnreadable, want: ErrUnreadable},
		{name: "recognizer unavailable", recognizer: errors.New("model down"), want: ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewService(
				&fakeLoader{image: Image{Data: []byte("png"), MIMEType: "image/png"}, err: tt.loaderErr},
				&fakeRecognizer{err: tt.recognizer},
			)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			_, err = service.Recognize(context.Background(), "image-ref", "expression")
			if !errors.Is(err, tt.want) {
				t.Fatalf("Recognize() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestServiceRecognizeValidatesStructuredResult(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   error
	}{
		{name: "explicit unreadable", result: Result{Status: "unreadable", Confidence: 0.2}, want: ErrUnreadable},
		{name: "low confidence", result: Result{Status: "ok", AnswerLatex: "x", Confidence: MinimumConfidence - 0.01}, want: ErrUnreadable},
		{name: "missing answer", result: Result{Status: "ok", Confidence: 1}, want: ErrUnavailable},
		{name: "oversized answer", result: Result{Status: "ok", AnswerLatex: strings.Repeat("x", MaxAnswerLength+1), Confidence: 1}, want: ErrUnavailable},
		{name: "unknown status", result: Result{Status: "maybe", AnswerLatex: "x", Confidence: 1}, want: ErrUnavailable},
		{name: "NaN confidence", result: Result{Status: "ok", AnswerLatex: "x", Confidence: math.NaN()}, want: ErrUnavailable},
		{name: "infinite confidence", result: Result{Status: "ok", AnswerLatex: "x", Confidence: math.Inf(1)}, want: ErrUnavailable},
		{name: "out of range confidence", result: Result{Status: "ok", AnswerLatex: "x", Confidence: 1.01}, want: ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewService(
				&fakeLoader{image: Image{Data: []byte("png"), MIMEType: "image/png"}},
				&fakeRecognizer{result: tt.result},
			)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			_, err = service.Recognize(context.Background(), "image-ref", "expression")
			if !errors.Is(err, tt.want) {
				t.Fatalf("Recognize() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestServiceRecognizePreservesCancellation(t *testing.T) {
	service, err := NewService(
		&fakeLoader{err: context.Canceled},
		&fakeRecognizer{},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.Recognize(context.Background(), "image-ref", "expression")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Recognize() error = %v, want context.Canceled", err)
	}
}

type fakeLoader struct {
	image     Image
	reference string
	err       error
}

func (l *fakeLoader) LoadImage(_ context.Context, reference string) (Image, error) {
	l.reference = reference
	if l.err != nil {
		return Image{}, l.err
	}
	return l.image, nil
}

type fakeRecognizer struct {
	result Result
	input  RecognizeInput
	called bool
	err    error
}

func (r *fakeRecognizer) Recognize(_ context.Context, input RecognizeInput) (Result, error) {
	r.called = true
	r.input = input
	if r.err != nil {
		return Result{}, r.err
	}
	return r.result, nil
}
