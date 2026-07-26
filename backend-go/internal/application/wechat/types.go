package wechat

import (
	"context"
	"errors"
	"time"
)

var (
	ErrUnavailable        = errors.New("wechat official account integration is unavailable")
	ErrBindingNotFound    = errors.New("wechat binding not found")
	ErrNotSubscribed      = errors.New("wechat account is not subscribed")
	ErrSendFailed         = errors.New("wechat message send failed")
	ErrCallbackInProgress = errors.New("wechat callback is already being processed")
	ErrCallbackClaimLost  = errors.New("wechat callback processing claim was lost")
	ErrOpenIDAlreadyBound = errors.New("wechat openid is already bound")
	ErrUserAlreadyBound   = errors.New("user already has a wechat binding")
)

// Config controls the WeChat Official Account application workflow.
type Config struct {
	Enabled            bool
	AppID              string
	AccountName        string
	BindingTicketTTL   time.Duration
	EventDedupeTTL     time.Duration
	EventProcessingTTL time.Duration
	TestMessage        string
}

// Binding is the internal persistence model for an Official Account binding.
type Binding struct {
	UserID     string
	OpenID     string
	Subscribed bool
	BoundAt    *time.Time
}

// Repository persists subscriptions and user bindings.
type Repository interface {
	GetByUserID(context.Context, string, string) (Binding, bool, error)
	Bind(context.Context, string, string, string, time.Time, time.Time) (Binding, error)
	SetSubscription(context.Context, string, string, bool, time.Time, time.Time) error
	Unbind(context.Context, string, string, time.Time) error
}

// StateStore owns short-lived, shared Redis state used by callback processing.
type StateStore interface {
	StoreBindingTicket(context.Context, string, string, time.Duration) (bool, error)
	ConsumeBindingTicket(context.Context, string, string) (string, bool, error)
	ClaimEvent(context.Context, string, string, time.Duration) (EventClaim, error)
	CompleteEvent(context.Context, string, string, string, time.Duration) (bool, error)
	ReleaseEvent(context.Context, string, string) error
}

// Sender sends a server-controlled text message to one Official Account user.
type Sender interface {
	SendText(context.Context, string, string) error
}

// BindingStatusResponse is returned to an authenticated student or teacher.
type BindingStatusResponse struct {
	Available   bool       `json:"available"`
	AccountName string     `json:"account_name,omitempty"`
	IsBound     bool       `json:"is_bound"`
	Subscribed  bool       `json:"subscribed"`
	BoundAt     *time.Time `json:"bound_at,omitempty"`
}

// BindingTicketResponse contains the short-lived command sent to the account.
type BindingTicketResponse struct {
	Ticket      string    `json:"ticket"`
	Command     string    `json:"command"`
	ExpiresAt   time.Time `json:"expires_at"`
	AccountName string    `json:"account_name,omitempty"`
}

// TestMessageResponse confirms acceptance by the WeChat API.
type TestMessageResponse struct {
	Sent bool `json:"sent"`
}

// IncomingMessage is the normalized callback shape consumed by the workflow.
type IncomingMessage struct {
	ToUserName   string
	FromUserName string
	CreateTime   int64
	MsgType      string
	Content      string
	MsgID        string
	Event        string
	EventKey     string
	Ticket       string
}

// ProcessResult controls the passive response to an inbound callback.
type ProcessResult struct {
	Reply     string
	Duplicate bool
}

// EventClaim describes whether a callback is new, currently running, or done.
type EventClaim struct {
	Acquired  bool
	Completed bool
	Reply     string
}
