package email

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrBadRequest is returned when email configuration or template input is invalid.
	ErrBadRequest = errors.New("bad email request")
	// ErrNotConfigured is returned when SMTP delivery is not ready for use.
	ErrNotConfigured = errors.New("email is not configured")
	// ErrDelivery is returned when the SMTP server rejects or cannot complete an operation.
	ErrDelivery = errors.New("email delivery failed")
)

// Error keeps a safe public message while retaining the internal cause for logs.
type Error struct {
	Kind    error
	Message string
	Cause   error
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Unwrap() error {
	if e.Cause != nil {
		return errors.Join(e.Kind, e.Cause)
	}
	return e.Kind
}

// Cipher protects the SMTP password at rest.
type Cipher interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

// Repository persists SMTP settings and administrator template overrides.
type Repository interface {
	GetSettings(context.Context, []string) (map[string]string, error)
	SaveSettings(context.Context, []SettingUpdate, bool) error
	ListTemplateOverrides(context.Context) ([]TemplateOverride, error)
	GetTemplateOverride(context.Context, string, string) (TemplateOverride, bool, error)
	UpsertTemplate(context.Context, TemplateOverride) error
	DeleteTemplate(context.Context, string, string) (bool, error)
}

// Transport owns SMTP protocol I/O.
type Transport interface {
	Test(context.Context, SMTPConfig) error
	Send(context.Context, SMTPConfig, Message) error
}

// EventSender is the notification boundary consumed by other application modules.
type EventSender interface {
	SendEvent(context.Context, EventRequest) error
}

// SettingUpdate stores one system setting mutation.
type SettingUpdate struct {
	Key         string
	Value       string
	Description string
	UpdatedAt   time.Time
}

// SMTPConfig is the complete secret-bearing configuration passed only to the transport.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	UseTLS   bool
}

// SMTPSettingsResponse is safe to return to administrators and never contains the password.
type SMTPSettingsResponse struct {
	SMTPHost               string `json:"smtp_host"`
	SMTPPort               int    `json:"smtp_port"`
	SMTPUsername           string `json:"smtp_username"`
	SMTPFrom               string `json:"smtp_from"`
	SMTPFromName           string `json:"smtp_from_name"`
	SMTPUseTLS             bool   `json:"smtp_use_tls"`
	SMTPPasswordConfigured bool   `json:"smtp_password_configured"`
	Configured             bool   `json:"configured"`
}

// UpdateSMTPSettingsInput replaces non-secret SMTP fields and optionally changes the password.
type UpdateSMTPSettingsInput struct {
	SMTPHost      string  `json:"smtp_host"`
	SMTPPort      int     `json:"smtp_port"`
	SMTPUsername  string  `json:"smtp_username"`
	SMTPPassword  *string `json:"smtp_password"`
	SMTPFrom      string  `json:"smtp_from"`
	SMTPFromName  string  `json:"smtp_from_name"`
	SMTPUseTLS    bool    `json:"smtp_use_tls"`
	ClearPassword bool    `json:"clear_password"`
}

// SMTPSettingsOverride carries optional draft values for connection and send tests.
type SMTPSettingsOverride struct {
	SMTPHost     *string `json:"smtp_host"`
	SMTPPort     *int    `json:"smtp_port"`
	SMTPUsername *string `json:"smtp_username"`
	SMTPPassword *string `json:"smtp_password"`
	SMTPFrom     *string `json:"smtp_from"`
	SMTPFromName *string `json:"smtp_from_name"`
	SMTPUseTLS   *bool   `json:"smtp_use_tls"`
}

// SendTestEmailInput sends one administrator-requested delivery using saved or draft settings.
type SendTestEmailInput struct {
	Recipient string `json:"recipient"`
	SMTPSettingsOverride
}

// ActionResponse is returned by connection tests and test deliveries.
type ActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Message is a fully rendered email.
type Message struct {
	To       string
	Subject  string
	HTMLBody string
}

// Event identifies one stable notification-template contract.
type Event string

const (
	EventPasswordReset      Event = "auth.password_reset"
	EventAccountSuspended   Event = "account.suspended"
	EventAccountDeactivated Event = "account.deactivated"
	EventAccountReactivated Event = "account.reactivated"
)

const (
	LocaleZhCN = "zh-CN"
	LocaleEnUS = "en-US"
)

// EventRequest asks the mail module to render and deliver one notification event.
type EventRequest struct {
	Event     Event
	Locale    string
	Recipient string
	Variables map[string]string
}

// NotificationResult reports a best-effort business notification without changing the main operation result.
type NotificationResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

const (
	NotificationSent    = "sent"
	NotificationFailed  = "failed"
	NotificationSkipped = "skipped"
)

// TemplateOverride is the persisted custom subject and body for one event/locale pair.
type TemplateOverride struct {
	Event     string
	Locale    string
	Subject   string
	HTMLBody  string
	UpdatedBy string
	UpdatedAt time.Time
}

// TemplateResponse describes either an official template or an administrator override.
type TemplateResponse struct {
	Event     string     `json:"event"`
	Locale    string     `json:"locale"`
	Name      string     `json:"name"`
	Subject   string     `json:"subject"`
	HTMLBody  string     `json:"html_body"`
	Variables []string   `json:"variables"`
	IsCustom  bool       `json:"is_custom"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// TemplateListResponse wraps every supported official template and its active override.
type TemplateListResponse struct {
	Items []TemplateResponse `json:"items"`
}

// TemplateUpdateInput replaces one administrator template override.
type TemplateUpdateInput struct {
	Subject  string `json:"subject"`
	HTMLBody string `json:"html_body"`
}

// TemplatePreviewInput renders either a draft or the currently active template with sample variables.
type TemplatePreviewInput struct {
	Event     string            `json:"event"`
	Locale    string            `json:"locale"`
	Subject   *string           `json:"subject"`
	HTMLBody  *string           `json:"html_body"`
	Variables map[string]string `json:"variables"`
}

// TemplatePreviewResponse contains a rendered subject and HTML document.
type TemplatePreviewResponse struct {
	Subject  string `json:"subject"`
	HTMLBody string `json:"html_body"`
}
