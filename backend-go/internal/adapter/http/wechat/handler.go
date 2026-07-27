package wechathttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	authapp "mathstudy/backend-go/internal/application/auth"
	wechatapp "mathstudy/backend-go/internal/application/wechat"
	"mathstudy/backend-go/internal/domain/user"
	wechatintegration "mathstudy/backend-go/internal/integration/wechat"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpvalidate"
	"mathstudy/backend-go/internal/platform/ratelimit"
	"mathstudy/backend-go/internal/platform/redact"
	"mathstudy/backend-go/internal/platform/securerand"
)

const (
	callbackClockSkew             = 5 * time.Minute
	callbackTotalBudget           = 4500 * time.Millisecond
	callbackBudget                = 3 * time.Second
	maxAdminBodyBytes             = 4 << 10
	bindingTicketRateLimitMax     = 3
	bindingTicketRateLimitWindow  = 10 * time.Minute
	bindingTicketRateLimitMaxKeys = 1000
	replyNonceAlphabet            = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// Service is the Official Account application surface used by HTTP handlers.
type Service interface {
	BindingStatus(context.Context, string) (wechatapp.BindingStatusResponse, error)
	CreateBindingTicket(context.Context, string) (wechatapp.BindingTicketResponse, error)
	Unbind(context.Context, string) error
	SendTestMessage(context.Context, string) (wechatapp.TestMessageResponse, error)
	ProcessIncoming(context.Context, wechatapp.IncomingMessage) (wechatapp.ProcessResult, error)
}

// Authenticator validates current access-token and account state.
type Authenticator interface {
	DecodeActiveAccessToken(context.Context, string) (authapp.Principal, bool, error)
}

// Config defines callback protocol behavior without exposing credentials.
type Config struct {
	Enabled     bool
	MessageMode string
}

// Handler serves public callbacks, user binding, and admin test sends.
type Handler struct {
	service  Service
	auth     Authenticator
	protocol *wechatintegration.Protocol
	config   Config
	logger   *slog.Logger
	limiter  *ratelimit.Limiter
	now      func() time.Time
}

// Option customizes the Official Account HTTP adapter.
type Option func(*Handler) error

// WithRedisRateLimit shares binding-ticket limits across API instances.
func WithRedisRateLimit(client *goredis.Client, maxLocalKeys int) Option {
	return func(handler *Handler) error {
		limiter, err := ratelimit.New(
			client,
			"msp:wechat:binding-ticket",
			bindingTicketRateLimitMax,
			bindingTicketRateLimitWindow,
			maxLocalKeys,
			handler.logger,
		)
		if err != nil {
			return err
		}
		handler.limiter = limiter
		return nil
	}
}

// NewHandler creates the Official Account HTTP adapter.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator, protocol *wechatintegration.Protocol, cfg Config, options ...Option) (*Handler, error) {
	if service == nil {
		return nil, errors.New("wechat service is nil")
	}
	if auth == nil {
		return nil, errors.New("wechat authenticator is nil")
	}
	if cfg.Enabled && protocol == nil {
		return nil, errors.New("wechat protocol is nil while integration is enabled")
	}
	if !cfg.Enabled {
		cfg.MessageMode = "plain"
	} else {
		switch cfg.MessageMode {
		case "plain", "compatible", "safe":
		default:
			return nil, errors.New("invalid wechat message mode")
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	handler := &Handler{
		service:  service,
		auth:     auth,
		protocol: protocol,
		config:   cfg,
		logger:   logger,
		limiter:  newBindingTicketRateLimiter(bindingTicketRateLimitMax, bindingTicketRateLimitWindow),
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(handler); err != nil {
			return nil, err
		}
	}
	return handler, nil
}

// RegisterPublic attaches the signature-protected callback route.
func (h *Handler) RegisterPublic(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/official-account/callback", h.verifyCallback)
	mux.Handle(
		"POST "+prefix+"/official-account/callback",
		http.TimeoutHandler(http.HandlerFunc(h.receiveCallback), callbackTotalBudget, "wechat callback timed out"),
	)
}

// RegisterUser attaches authenticated student and teacher binding routes.
func (h *Handler) RegisterUser(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/binding", h.bindingStatus)
	mux.HandleFunc("POST "+prefix+"/binding-ticket", h.createBindingTicket)
	mux.HandleFunc("DELETE "+prefix+"/binding", h.unbind)
}

// RegisterAdmin attaches the fixed single-user test-send route.
func (h *Handler) RegisterAdmin(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix+"/test-message", h.sendTestMessage)
}

func (h *Handler) verifyCallback(w http.ResponseWriter, r *http.Request) {
	if !h.callbackAvailable(w) {
		return
	}
	if !h.validTimestamp(r.URL.Query().Get("timestamp")) {
		writeCallbackError(w, http.StatusForbidden)
		return
	}
	query := r.URL.Query()
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")
	echo := query.Get("echostr")
	if echo == "" || len(echo) > 128<<10 {
		writeCallbackError(w, http.StatusBadRequest)
		return
	}
	encrypted := query.Get("encrypt_type") == "aes" || query.Get("msg_signature") != ""
	if encrypted {
		if !h.acceptsEncrypted() || h.protocol.VerifyMessageSignature(query.Get("msg_signature"), timestamp, nonce, echo) != nil {
			writeCallbackError(w, http.StatusForbidden)
			return
		}
		plain, err := h.protocol.Decrypt(echo)
		if err != nil {
			writeCallbackError(w, http.StatusForbidden)
			return
		}
		h.writePlain(w, plain)
		return
	}
	if !h.acceptsPlaintext() || h.protocol.VerifySignature(query.Get("signature"), timestamp, nonce) != nil {
		writeCallbackError(w, http.StatusForbidden)
		return
	}
	h.writePlain(w, []byte(echo))
}

func (h *Handler) receiveCallback(w http.ResponseWriter, r *http.Request) {
	if !h.callbackAvailable(w) {
		return
	}
	if !h.validTimestamp(r.URL.Query().Get("timestamp")) {
		writeCallbackError(w, http.StatusForbidden)
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, wechatintegration.MaxCallbackBodyBytes))
	if err != nil {
		writeCallbackError(w, http.StatusRequestEntityTooLarge)
		return
	}
	defer r.Body.Close()

	query := r.URL.Query()
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")
	encrypted := query.Get("encrypt_type") == "aes" || query.Get("msg_signature") != ""
	var incoming wechatintegration.IncomingMessage
	if encrypted {
		if !h.acceptsEncrypted() {
			writeCallbackError(w, http.StatusForbidden)
			return
		}
		envelope, parseErr := wechatintegration.ParseEncryptedEnvelopeXML(payload)
		if parseErr != nil || h.protocol.VerifyMessageSignature(query.Get("msg_signature"), timestamp, nonce, envelope.Encrypt) != nil {
			writeCallbackError(w, http.StatusForbidden)
			return
		}
		plaintext, decryptErr := h.protocol.Decrypt(envelope.Encrypt)
		if decryptErr != nil {
			writeCallbackError(w, http.StatusForbidden)
			return
		}
		incoming, err = wechatintegration.ParseIncomingXML(plaintext)
	} else {
		if !h.acceptsPlaintext() || h.protocol.VerifySignature(query.Get("signature"), timestamp, nonce) != nil {
			writeCallbackError(w, http.StatusForbidden)
			return
		}
		incoming, err = wechatintegration.ParseIncomingXML(payload)
	}
	if err != nil {
		writeCallbackError(w, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), callbackBudget)
	defer cancel()
	result, err := h.service.ProcessIncoming(ctx, wechatapp.IncomingMessage{
		ToUserName:   incoming.ToUserName,
		FromUserName: incoming.FromUserName,
		CreateTime:   incoming.CreateTime,
		MsgType:      incoming.MsgType,
		Content:      incoming.Content,
		MsgID:        incoming.MsgID,
		Event:        incoming.Event,
		EventKey:     incoming.EventKey,
		Ticket:       incoming.Ticket,
	})
	if err != nil {
		if errors.Is(err, wechatapp.ErrCallbackInProgress) {
			writeCallbackError(w, http.StatusServiceUnavailable)
			return
		}
		h.logger.Error("wechat callback processing failed", "error", redact.String(err.Error()))
		writeCallbackError(w, http.StatusInternalServerError)
		return
	}
	if result.Reply == "" {
		h.writePlain(w, []byte("success"))
		return
	}
	replyTimestamp := h.now().Unix()
	reply, err := wechatintegration.BuildTextReply(incoming, result.Reply, replyTimestamp)
	if err != nil {
		h.logger.Error("build wechat callback reply", "error", redact.String(err.Error()))
		writeCallbackError(w, http.StatusInternalServerError)
		return
	}
	if encrypted {
		replyNonce, nonceErr := securerand.String(16, replyNonceAlphabet)
		if nonceErr != nil {
			h.logger.Error("generate wechat callback reply nonce", "error", nonceErr)
			writeCallbackError(w, http.StatusInternalServerError)
			return
		}
		reply, err = h.protocol.BuildEncryptedReply(reply, replyTimestamp, replyNonce)
		if err != nil {
			h.logger.Error("encrypt wechat callback reply", "error", redact.String(err.Error()))
			writeCallbackError(w, http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(reply)
}

func (h *Handler) bindingStatus(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireBindingUser(w, r)
	if !ok {
		return
	}
	response, err := h.service.BindingStatus(r.Context(), principal.UserID)
	if err != nil {
		h.writeServiceError(w, err, "获取微信绑定状态失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) createBindingTicket(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireBindingUser(w, r)
	if !ok {
		return
	}
	if h.limiter != nil && !h.limiter.Allow(r.Context(), principal.UserID) {
		w.Header().Set("Retry-After", strconv.Itoa(int(bindingTicketRateLimitWindow/time.Second)))
		httpjson.WriteDetailError(w, http.StatusTooManyRequests, "WECHAT_BINDING_RATE_LIMITED", "绑定口令生成过于频繁，请稍后重试")
		return
	}
	response, err := h.service.CreateBindingTicket(r.Context(), principal.UserID)
	if err != nil {
		h.writeServiceError(w, err, "生成微信绑定口令失败")
		return
	}
	httpjson.Write(w, http.StatusCreated, response)
}

func (h *Handler) unbind(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireBindingUser(w, r)
	if !ok {
		return
	}
	if err := h.service.Unbind(r.Context(), principal.UserID); err != nil {
		h.writeServiceError(w, err, "解绑微信失败")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type testMessageRequest struct {
	UserID string `json:"user_id"`
}

func (h *Handler) sendTestMessage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request testMessageRequest
	if !httpjson.DecodeStrictOrDetailError(w, r, maxAdminBodyBytes, &request) {
		return
	}
	request.UserID = strings.TrimSpace(request.UserID)
	if !httpvalidate.RequiredTrimmedString(w, request.UserID, 1, 64, "user_id", httpjson.WriteDetailError) {
		return
	}
	response, err := h.service.SendTestMessage(r.Context(), request.UserID)
	if err != nil {
		h.writeServiceError(w, err, "发送微信测试消息失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requireBindingUser(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return h.requireRole(
		w,
		r,
		func(principal authapp.Principal) bool {
			return authapp.HasAnyRole(principal, user.RoleStudent, user.RoleTeacher)
		},
		"权限不足，仅学生或教师可以管理微信绑定",
	)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return h.requireRole(w, r, authapp.IsAdmin, "权限不足，需要管理员权限")
}

func (h *Handler) requireRole(w http.ResponseWriter, r *http.Request, allow func(authapp.Principal) bool, forbidden string) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccessContext(
		w,
		r,
		h.auth.DecodeActiveAccessToken,
		allow,
		forbidden,
		httpjson.WriteDetailError,
		func(err error) {
			h.logger.Error("wechat access-token validation failed", "error", redact.String(err.Error()))
		},
	)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, wechatapp.ErrUnavailable):
		httpjson.WriteDetailError(w, http.StatusServiceUnavailable, "WECHAT_UNAVAILABLE", "微信公众号功能尚未启用")
	case errors.Is(err, wechatapp.ErrBindingNotFound):
		httpjson.WriteDetailError(w, http.StatusConflict, "WECHAT_NOT_BOUND", "目标用户尚未绑定微信")
	case errors.Is(err, wechatapp.ErrNotSubscribed):
		httpjson.WriteDetailError(w, http.StatusConflict, "WECHAT_NOT_SUBSCRIBED", "目标用户已取消关注公众号")
	case errors.Is(err, wechatapp.ErrSendFailed):
		h.logger.Error("wechat test-message send failed", "error", redact.String(err.Error()))
		httpjson.WriteDetailError(w, http.StatusBadGateway, "WECHAT_SEND_FAILED", "微信未接受测试消息，请确认接口权限和客服消息时限")
	default:
		h.logger.Error("wechat request failed", "error", redact.String(err.Error()))
		httpjson.WriteDetailError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func (h *Handler) callbackAvailable(w http.ResponseWriter) bool {
	if h.config.Enabled && h.protocol != nil {
		return true
	}
	http.Error(w, "not found", http.StatusNotFound)
	return false
}

func (h *Handler) acceptsPlaintext() bool {
	return h.config.MessageMode == "plain" || h.config.MessageMode == "compatible"
}

func (h *Handler) acceptsEncrypted() bool {
	return h.protocol != nil && h.protocol.HasCipher() && (h.config.MessageMode == "compatible" || h.config.MessageMode == "safe")
}

func (h *Handler) validTimestamp(value string) bool {
	timestamp, err := strconv.ParseInt(value, 10, 64)
	if err != nil || timestamp <= 0 {
		return false
	}
	delta := h.now().Sub(time.Unix(timestamp, 0))
	return delta >= -callbackClockSkew && delta <= callbackClockSkew
}

func (h *Handler) writePlain(w http.ResponseWriter, payload []byte) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func writeCallbackError(w http.ResponseWriter, status int) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, fmt.Sprintf("wechat callback rejected (%d)", status), status)
}

func newBindingTicketRateLimiter(limit int, window time.Duration) *ratelimit.Limiter {
	if limit <= 0 {
		limit = bindingTicketRateLimitMax
	}
	if window <= 0 {
		window = bindingTicketRateLimitWindow
	}
	limiter, err := ratelimit.New(nil, "msp:wechat:binding-ticket", limit, window, bindingTicketRateLimitMaxKeys, nil)
	if err != nil {
		panic(err)
	}
	return limiter
}
