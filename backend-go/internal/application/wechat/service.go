package wechat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"mathstudy/backend-go/internal/platform/securerand"
)

const (
	defaultBindingTicketTTL   = 10 * time.Minute
	defaultEventDedupeTTL     = 24 * time.Hour
	defaultEventProcessingTTL = 6 * time.Second
	eventCompletionBudget     = 500 * time.Millisecond
	eventReleaseBudget        = 250 * time.Millisecond
	defaultAccountName        = "微信公众号"
	defaultTestMessage        = "平台公众号通道测试成功。"
	bindingAlphabet           = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	eventOwnerAlphabet        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	maxTicketCreateAttempts   = 5
)

// Service implements Official Account binding and callback use cases.
type Service struct {
	repository Repository
	state      StateStore
	sender     Sender
	config     Config
	now        func() time.Time
}

// NewService creates the Official Account application service.
func NewService(repository Repository, state StateStore, sender Sender, cfg Config) (*Service, error) {
	if repository == nil {
		return nil, errors.New("wechat repository is nil")
	}
	if state == nil {
		return nil, errors.New("wechat state store is nil")
	}
	if sender == nil && cfg.Enabled {
		return nil, errors.New("wechat sender is nil")
	}
	if cfg.BindingTicketTTL <= 0 {
		cfg.BindingTicketTTL = defaultBindingTicketTTL
	}
	if cfg.EventDedupeTTL <= 0 {
		cfg.EventDedupeTTL = defaultEventDedupeTTL
	}
	if cfg.EventProcessingTTL <= 0 {
		cfg.EventProcessingTTL = defaultEventProcessingTTL
	}
	if cfg.EventProcessingTTL >= cfg.EventDedupeTTL {
		return nil, errors.New("wechat event processing TTL must be shorter than dedupe TTL")
	}
	if strings.TrimSpace(cfg.AccountName) == "" {
		cfg.AccountName = defaultAccountName
	}
	if strings.TrimSpace(cfg.TestMessage) == "" {
		cfg.TestMessage = defaultTestMessage
	}
	return &Service{
		repository: repository,
		state:      state,
		sender:     sender,
		config:     cfg,
		now:        func() time.Time { return time.Now().UTC() },
	}, nil
}

// BindingStatus returns the current user's Official Account binding.
func (s *Service) BindingStatus(ctx context.Context, userID string) (BindingStatusResponse, error) {
	response := BindingStatusResponse{
		Available:   s.config.Enabled,
		AccountName: s.config.AccountName,
	}
	if !s.config.Enabled {
		return response, nil
	}
	binding, found, err := s.repository.GetByUserID(ctx, s.config.AppID, userID)
	if err != nil {
		return BindingStatusResponse{}, fmt.Errorf("get wechat binding: %w", err)
	}
	if !found {
		return response, nil
	}
	response.IsBound = true
	response.Subscribed = binding.Subscribed
	response.BoundAt = binding.BoundAt
	return response, nil
}

// CreateBindingTicket creates a one-time command valid for a short period.
func (s *Service) CreateBindingTicket(ctx context.Context, userID string) (BindingTicketResponse, error) {
	if !s.config.Enabled {
		return BindingTicketResponse{}, ErrUnavailable
	}
	for range maxTicketCreateAttempts {
		raw, err := securerand.String(8, bindingAlphabet)
		if err != nil {
			return BindingTicketResponse{}, fmt.Errorf("generate wechat binding ticket: %w", err)
		}
		ticket := raw[:4] + "-" + raw[4:]
		stored, err := s.state.StoreBindingTicket(ctx, ticketDigest(s.config.AppID, raw), userID, s.config.BindingTicketTTL)
		if err != nil {
			return BindingTicketResponse{}, fmt.Errorf("store wechat binding ticket: %w", err)
		}
		if !stored {
			continue
		}
		return BindingTicketResponse{
			Ticket:      ticket,
			Command:     "绑定 " + ticket,
			ExpiresAt:   s.now().Add(s.config.BindingTicketTTL),
			AccountName: s.config.AccountName,
		}, nil
	}
	return BindingTicketResponse{}, errors.New("generate unique wechat binding ticket")
}

// Unbind removes the user's platform association while retaining subscribe state.
func (s *Service) Unbind(ctx context.Context, userID string) error {
	if !s.config.Enabled {
		return ErrUnavailable
	}
	if err := s.repository.Unbind(ctx, s.config.AppID, userID, s.now()); err != nil {
		return fmt.Errorf("unbind wechat account: %w", err)
	}
	return nil
}

// SendTestMessage sends fixed server-owned text to one bound user.
func (s *Service) SendTestMessage(ctx context.Context, userID string) (TestMessageResponse, error) {
	if !s.config.Enabled {
		return TestMessageResponse{}, ErrUnavailable
	}
	binding, found, err := s.repository.GetByUserID(ctx, s.config.AppID, userID)
	if err != nil {
		return TestMessageResponse{}, fmt.Errorf("get test-message binding: %w", err)
	}
	if !found {
		return TestMessageResponse{}, ErrBindingNotFound
	}
	if !binding.Subscribed {
		return TestMessageResponse{}, ErrNotSubscribed
	}
	if err := s.sender.SendText(ctx, binding.OpenID, s.config.TestMessage); err != nil {
		return TestMessageResponse{}, fmt.Errorf("%w: %v", ErrSendFailed, err)
	}
	return TestMessageResponse{Sent: true}, nil
}

// ProcessIncoming applies a verified, normalized WeChat callback once.
func (s *Service) ProcessIncoming(ctx context.Context, message IncomingMessage) (result ProcessResult, err error) {
	if !s.config.Enabled {
		return ProcessResult{}, ErrUnavailable
	}
	eventKey := callbackEventKey(s.config.AppID, message)
	owner, err := securerand.String(24, eventOwnerAlphabet)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("generate wechat callback owner: %w", err)
	}
	claim, err := s.state.ClaimEvent(ctx, eventKey, owner, s.config.EventProcessingTTL)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("claim wechat callback: %w", err)
	}
	if claim.Completed {
		return ProcessResult{Reply: claim.Reply, Duplicate: true}, nil
	}
	if !claim.Acquired {
		return ProcessResult{}, ErrCallbackInProgress
	}
	finalized := false
	defer func() {
		if finalized {
			return
		}
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), eventReleaseBudget)
		defer cancel()
		if releaseErr := s.state.ReleaseEvent(releaseCtx, eventKey, owner); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("release failed wechat callback claim: %w", releaseErr))
		}
	}()

	switch strings.ToLower(strings.TrimSpace(message.MsgType)) {
	case "event":
		result, err = s.processEvent(ctx, message)
	case "text":
		result, err = s.processText(ctx, message)
	default:
		result = ProcessResult{}
	}
	if err != nil {
		return result, err
	}
	completeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), eventCompletionBudget)
	defer cancel()
	completed, completeErr := s.state.CompleteEvent(completeCtx, eventKey, owner, result.Reply, s.config.EventDedupeTTL)
	if completeErr != nil {
		return ProcessResult{}, fmt.Errorf("complete wechat callback: %w", completeErr)
	}
	if !completed {
		return ProcessResult{}, ErrCallbackClaimLost
	}
	finalized = true
	return result, nil
}

func (s *Service) processEvent(ctx context.Context, message IncomingMessage) (ProcessResult, error) {
	processedAt := s.now()
	observedAt := messageObservedAt(message, processedAt)
	switch strings.ToLower(strings.TrimSpace(message.Event)) {
	case "subscribe":
		if err := s.repository.SetSubscription(ctx, s.config.AppID, message.FromUserName, true, observedAt, processedAt); err != nil {
			return ProcessResult{}, fmt.Errorf("record wechat subscription: %w", err)
		}
		return ProcessResult{Reply: "感谢关注。请登录平台，在个人中心查看或管理微信公众号绑定。"}, nil
	case "unsubscribe":
		if err := s.repository.SetSubscription(ctx, s.config.AppID, message.FromUserName, false, observedAt, processedAt); err != nil {
			return ProcessResult{}, fmt.Errorf("record wechat unsubscription: %w", err)
		}
	}
	return ProcessResult{}, nil
}

func (s *Service) processText(ctx context.Context, message IncomingMessage) (ProcessResult, error) {
	ticket, ok := parseBindingCommand(message.Content)
	if !ok {
		return ProcessResult{}, nil
	}
	digest := ticketDigest(s.config.AppID, ticket)
	userID, found, err := s.state.ConsumeBindingTicket(ctx, digest, callbackEventKey(s.config.AppID, message))
	if err != nil {
		return ProcessResult{}, fmt.Errorf("consume wechat binding ticket: %w", err)
	}
	if !found {
		return ProcessResult{Reply: "绑定口令无效或已过期，请回到平台个人中心重新生成。"}, nil
	}
	processedAt := s.now()
	observedAt := messageObservedAt(message, processedAt)
	if _, err := s.repository.Bind(ctx, s.config.AppID, message.FromUserName, userID, observedAt, processedAt); err != nil {
		switch {
		case errors.Is(err, ErrOpenIDAlreadyBound):
			return ProcessResult{Reply: "该微信已绑定其他平台账号，请先在原账号中解绑。"}, nil
		case errors.Is(err, ErrUserAlreadyBound):
			return ProcessResult{Reply: "该平台账号已绑定其他微信，请先在个人中心解绑。"}, nil
		default:
			return ProcessResult{}, fmt.Errorf("bind wechat account: %w", err)
		}
	}
	return ProcessResult{Reply: "绑定成功。你现在可以通过本公众号接收平台消息提醒。"}, nil
}

func parseBindingCommand(content string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(content))
	if len(fields) != 2 || fields[0] != "绑定" {
		return "", false
	}
	ticket := strings.ToUpper(strings.ReplaceAll(fields[1], "-", ""))
	if len(ticket) != 8 {
		return "", false
	}
	for _, char := range ticket {
		if !strings.ContainsRune(bindingAlphabet, char) {
			return "", false
		}
	}
	return ticket, true
}

func ticketDigest(appID, normalizedTicket string) string {
	digest := sha256.Sum256([]byte(appID + "\x00" + strings.ToUpper(strings.ReplaceAll(normalizedTicket, "-", ""))))
	return hex.EncodeToString(digest[:])
}

func callbackEventKey(appID string, message IncomingMessage) string {
	parts := []string{appID}
	if strings.TrimSpace(message.MsgID) != "" {
		parts = append(parts, "message", strings.TrimSpace(message.MsgID))
	} else {
		parts = append(parts,
			"event",
			message.ToUserName,
			message.FromUserName,
			fmt.Sprintf("%d", message.CreateTime),
			strings.ToLower(strings.TrimSpace(message.MsgType)),
			strings.ToLower(strings.TrimSpace(message.Event)),
			message.EventKey,
			message.Ticket,
			message.Content,
		)
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

func messageObservedAt(message IncomingMessage, fallback time.Time) time.Time {
	if message.CreateTime <= 0 {
		return fallback
	}
	return time.Unix(message.CreateTime, 0).UTC()
}
