package airisk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Acquire checks student access, content, quota, and distributed concurrency.
// Metered requests reserve quota capacity until their lease is released.
func (s *Service) Acquire(ctx context.Context, studentID, source, content string, metered bool) (Lease, error) {
	access, ok, err := s.repo.GetStudentAccess(ctx, strings.TrimSpace(studentID))
	if err != nil {
		return nil, Error{Kind: ErrUnavailable, Message: "AI 风控服务暂不可用"}
	}
	if !ok || !access.IsStudent {
		return noopLease{}, nil
	}
	if access.IsBlocked {
		message := "你的 AI 使用权限已被管理员暂停"
		if access.BlockedReason != "" {
			message += "：" + access.BlockedReason
		}
		return nil, Error{Kind: ErrAccessBlocked, Message: message}
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, Error{Kind: ErrUnavailable, Message: "AI 风控服务暂不可用"}
	}
	if matched := matchBlockedKeyword(content, settings.BlockedKeywords); matched != "" {
		recordErr := s.recordContentBlock(ctx, access, source, content, matched)
		blocked := Error{Kind: ErrContentBlocked, Message: "该内容触发平台安全规则，请调整后重试"}
		if recordErr != nil {
			return nil, errors.Join(blocked, recordErr)
		}
		return nil, blocked
	}
	usedToday := 0
	if metered {
		usedToday, err = s.repo.CountReplies(ctx, access.StudentID, s.usageDate(s.now()))
		if err != nil {
			return nil, Error{Kind: ErrUnavailable, Message: "AI 风控服务暂不可用"}
		}
		if usedToday >= settings.DailyReplyLimit {
			return nil, Error{Kind: ErrQuotaExceeded, Message: "今日 AI 回复额度已用完，请明天再试"}
		}
	}
	leaseID, err := s.newID()
	if err != nil {
		return nil, Error{Kind: ErrUnavailable, Message: "AI 风控服务暂不可用"}
	}
	dailyLimit := 0
	if metered {
		dailyLimit = settings.DailyReplyLimit
	}
	decision, err := s.slots.Acquire(
		ctx,
		access.StudentID,
		leaseID,
		settings.MaxConcurrentRequests,
		dailyLimit,
		usedToday,
		s.leaseTTL,
	)
	if err != nil {
		return nil, Error{Kind: ErrUnavailable, Message: "AI 风控服务暂不可用"}
	}
	if !decision.Allowed {
		if decision.Reason == "quota" {
			return nil, Error{Kind: ErrQuotaExceeded, Message: "今日 AI 回复额度已用完，请明天再试"}
		}
		return nil, Error{Kind: ErrConcurrencyExceeded, Message: "已有 AI 请求处理中，请等待完成后再试"}
	}
	return &distributedLease{store: s.slots, studentID: access.StudentID, leaseID: leaseID}, nil
}

func (s *Service) recordContentBlock(ctx context.Context, access StudentAccess, source, content, matched string) error {
	eventID, err := s.newID()
	if err != nil {
		return fmt.Errorf("create content risk event ID: %w", err)
	}
	now := s.now()
	studentID := access.StudentID
	digest := sha256.Sum256([]byte(content))
	return s.repo.InsertRiskEvent(ctx, RiskEvent{
		ID:              eventID,
		StudentID:       &studentID,
		StudentUsername: access.Username,
		EventType:       "content_blocked",
		Severity:        "critical",
		Action:          "request_blocked",
		Source:          strings.TrimSpace(source),
		MatchedRule:     matched,
		ContentExcerpt:  contentExcerpt(content, 240),
		ContentHash:     hex.EncodeToString(digest[:]),
		EventDate:       s.usageDate(now),
		CreatedAt:       now,
	})
}

func matchBlockedKeyword(content string, keywords []string) string {
	content = strings.ToLower(strings.TrimSpace(content))
	if content == "" {
		return ""
	}
	for _, keyword := range keywords {
		if normalized := strings.ToLower(strings.TrimSpace(keyword)); normalized != "" && strings.Contains(content, normalized) {
			return keyword
		}
	}
	return ""
}

func contentExcerpt(content string, limit int) string {
	content = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, content))
	runes := []rune(content)
	if len(runes) <= limit {
		return content
	}
	return string(runes[:limit])
}

// SetLeaseTTLForTest is intentionally package-private through tests in this package.
func (s *Service) setLeaseTTLForTest(ttl time.Duration) { s.leaseTTL = ttl }
