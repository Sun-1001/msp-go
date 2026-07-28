package wechatreminder

import (
	"context"
	"time"
)

// EventType identifies the message-center event represented by one reminder.
type EventType string

const (
	EventPrivateMessage EventType = "private_message"
	EventNotice         EventType = "notice"
	EventQAMessage      EventType = "qa_message"
)

// Job is a leased, content-free reminder task.
type Job struct {
	ID              int64
	EventType       EventType
	SourceID        string
	RecipientUserID string
	AttemptCount    int
}

// Delivery is resolved immediately before a send and is never persisted in the job.
type Delivery struct {
	OpenID     string
	ActorName  string
	Content    string
	OccurredAt time.Time
}

// Repository owns durable reminder leases and delivery eligibility checks.
type Repository interface {
	Claim(context.Context, string, string, time.Time, time.Time, int) ([]Job, error)
	ResolveDelivery(context.Context, string, Job) (Delivery, bool, string, error)
	RenewLease(context.Context, int64, string, time.Time, time.Time) (bool, error)
	MarkSent(context.Context, int64, string, time.Time) (bool, error)
	MarkSkipped(context.Context, int64, string, string, *int, time.Time) (bool, error)
	Reschedule(context.Context, int64, string, string, *int, time.Time) (bool, error)
	MarkDead(context.Context, int64, string, string, *int, time.Time) (bool, error)
	DeleteFinishedBefore(context.Context, string, time.Time, int) (int64, error)
}

// Sender sends one server-controlled template message to an eligible OpenID.
type Sender interface {
	SendTemplate(context.Context, string, string, map[string]string) error
}

// ProviderError exposes only credential-free fields needed for retry decisions.
type ProviderError interface {
	error
	WechatProviderCode() int
	WechatHTTPStatus() int
	WechatRetryable() bool
}

// FailureDisposition determines the terminal or retry transition after a send error.
type FailureDisposition string

const (
	FailureRetry FailureDisposition = "retry"
	FailureSkip  FailureDisposition = "skip"
	FailureDead  FailureDisposition = "dead"
)

// SendFailure is a safe classification that may be persisted and logged.
type SendFailure struct {
	Disposition  FailureDisposition
	Code         string
	ProviderCode *int
}

// ErrorClassifier converts an outbound error without exposing its text.
type ErrorClassifier func(error) SendFailure
