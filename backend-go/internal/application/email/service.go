package email

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"

	"mathstudy/backend-go/internal/platform/redact"
)

const (
	settingSMTPHost     = "smtp_host"
	settingSMTPPort     = "smtp_port"
	settingSMTPUsername = "smtp_username"
	settingSMTPPassword = "smtp_password"
	settingSMTPFrom     = "smtp_from"
	settingSMTPFromName = "smtp_from_name"
	settingSMTPUseTLS   = "smtp_use_tls"
	settingSystemName   = "system_name"

	defaultSMTPPort = 587
	defaultSystem   = "高等数学智能学习平台"
	maxPasswordSize = 4096
)

var smtpSettingKeys = []string{
	settingSMTPHost,
	settingSMTPPort,
	settingSMTPUsername,
	settingSMTPPassword,
	settingSMTPFrom,
	settingSMTPFromName,
	settingSMTPUseTLS,
	settingSystemName,
}

// Service implements SMTP configuration, template management, and event delivery.
type Service struct {
	repo      Repository
	cipher    Cipher
	transport Transport
	logger    *slog.Logger
	now       func() time.Time
}

// NewService creates the email application service.
func NewService(repo Repository, cipher Cipher, transport Transport, logger *slog.Logger) (*Service, error) {
	if repo == nil {
		return nil, errors.New("email repository is nil")
	}
	if cipher == nil {
		return nil, errors.New("email cipher is nil")
	}
	if transport == nil {
		return nil, errors.New("email transport is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:      repo,
		cipher:    cipher,
		transport: transport,
		logger:    logger,
		now:       func() time.Time { return time.Now().UTC() },
	}, nil
}

// Settings returns password-safe SMTP settings for the administrator UI.
func (s *Service) Settings(ctx context.Context) (SMTPSettingsResponse, error) {
	config, passwordConfigured, _, err := s.loadStoredConfig(ctx, false)
	if err != nil {
		return SMTPSettingsResponse{}, err
	}
	return settingsResponse(config, passwordConfigured), nil
}

// UpdateSettings validates and atomically persists SMTP settings.
func (s *Service) UpdateSettings(ctx context.Context, input UpdateSMTPSettingsInput) (SMTPSettingsResponse, error) {
	config := SMTPConfig{
		Host:     strings.TrimSpace(input.SMTPHost),
		Port:     input.SMTPPort,
		Username: strings.TrimSpace(input.SMTPUsername),
		From:     strings.TrimSpace(input.SMTPFrom),
		FromName: strings.TrimSpace(input.SMTPFromName),
		UseTLS:   input.SMTPUseTLS,
	}
	if err := validateDeliveryConfig(config); err != nil {
		return SMTPSettingsResponse{}, err
	}
	if input.SMTPPassword != nil && len(*input.SMTPPassword) > maxPasswordSize {
		return SMTPSettingsResponse{}, badRequest("smtp_password 长度不能超过 4096 字节")
	}
	newPassword := input.SMTPPassword != nil && *input.SMTPPassword != ""
	if input.ClearPassword && newPassword {
		return SMTPSettingsResponse{}, badRequest("不能同时设置和清除 smtp_password")
	}
	if config.Username == "" && newPassword {
		return SMTPSettingsResponse{}, badRequest("设置 smtp_password 时 smtp_username 不能为空")
	}

	values, err := s.repo.GetSettings(ctx, []string{settingSMTPPassword})
	if err != nil {
		return SMTPSettingsResponse{}, fmt.Errorf("get stored smtp password state: %w", err)
	}
	passwordConfigured := strings.TrimSpace(values[settingSMTPPassword]) != ""
	if config.Username == "" && passwordConfigured && !input.ClearPassword && !newPassword {
		return SMTPSettingsResponse{}, badRequest("清空 smtp_username 时必须同时清除 smtp_password")
	}
	now := s.now()
	updates := []SettingUpdate{
		{Key: settingSMTPHost, Value: config.Host, Description: "SMTP 服务器地址", UpdatedAt: now},
		{Key: settingSMTPPort, Value: strconv.Itoa(config.Port), Description: "SMTP 服务器端口", UpdatedAt: now},
		{Key: settingSMTPUsername, Value: config.Username, Description: "SMTP 登录用户名", UpdatedAt: now},
		{Key: settingSMTPFrom, Value: config.From, Description: "SMTP 发件邮箱", UpdatedAt: now},
		{Key: settingSMTPFromName, Value: config.FromName, Description: "SMTP 发件人名称", UpdatedAt: now},
		{Key: settingSMTPUseTLS, Value: strconv.FormatBool(config.UseTLS), Description: "SMTP 是否使用直接 TLS", UpdatedAt: now},
	}
	if newPassword {
		encrypted, err := s.cipher.Encrypt(*input.SMTPPassword)
		if err != nil {
			return SMTPSettingsResponse{}, fmt.Errorf("encrypt smtp password: %w", err)
		}
		updates = append(updates, SettingUpdate{
			Key:         settingSMTPPassword,
			Value:       encrypted,
			Description: "SMTP 登录密码（Fernet 加密）",
			UpdatedAt:   now,
		})
		passwordConfigured = true
	}
	if input.ClearPassword {
		passwordConfigured = false
	}
	if err := s.repo.SaveSettings(ctx, updates, input.ClearPassword); err != nil {
		return SMTPSettingsResponse{}, fmt.Errorf("save smtp settings: %w", err)
	}
	return settingsResponse(config, passwordConfigured), nil
}

// TestSMTP authenticates against saved settings with optional unsaved draft overrides.
func (s *Service) TestSMTP(ctx context.Context, override SMTPSettingsOverride) (ActionResponse, error) {
	config, _, err := s.resolveConfig(ctx, override)
	if err != nil {
		return ActionResponse{}, err
	}
	if err := validateConnectionConfig(config); err != nil {
		return ActionResponse{}, err
	}
	if err := s.transport.Test(ctx, config); err != nil {
		return ActionResponse{}, deliveryError("SMTP 连接失败，请检查服务器、端口、账号和加密方式", err)
	}
	return ActionResponse{Success: true, Message: "SMTP 连接成功"}, nil
}

// SendTestEmail sends one test message with saved settings and optional draft overrides.
func (s *Service) SendTestEmail(ctx context.Context, input SendTestEmailInput) (ActionResponse, error) {
	config, systemName, err := s.resolveConfig(ctx, input.SMTPSettingsOverride)
	if err != nil {
		return ActionResponse{}, err
	}
	if err := validateDeliveryConfig(config); err != nil {
		return ActionResponse{}, err
	}
	recipient, err := normalizeEmail(input.Recipient, "recipient")
	if err != nil {
		return ActionResponse{}, err
	}
	message := Message{
		To:      recipient,
		Subject: systemName + " 邮件服务测试",
		HTMLBody: emailDocument(`<h1>邮件服务测试成功</h1><p>这是来自 <strong>` +
			html.EscapeString(systemName) + `</strong> 的测试邮件。</p>`),
	}
	if err := s.transport.Send(ctx, config, message); err != nil {
		return ActionResponse{}, deliveryError("测试邮件发送失败，请检查 SMTP 配置和收件地址", err)
	}
	return ActionResponse{Success: true, Message: "测试邮件已发送"}, nil
}

// SendEvent renders and sends one supported business notification.
func (s *Service) SendEvent(ctx context.Context, request EventRequest) error {
	definition, err := normalizeTemplateIdentity(string(request.Event), request.Locale)
	if err != nil {
		return err
	}
	recipient, err := normalizeEmail(request.Recipient, "recipient")
	if err != nil {
		return err
	}
	config, _, systemName, err := s.loadStoredConfig(ctx, true)
	if err != nil {
		return err
	}
	if strings.TrimSpace(config.Host) == "" || strings.TrimSpace(config.From) == "" {
		return notConfigured("邮件服务尚未配置")
	}
	if err := validateDeliveryConfig(config); err != nil {
		return err
	}
	override, found, err := s.repo.GetTemplateOverride(ctx, string(definition.Event), definition.Locale)
	if err != nil {
		return fmt.Errorf("get email template override: %w", err)
	}
	subject, htmlBody := definition.Subject, definition.HTMLBody
	if found {
		subject, htmlBody = override.Subject, override.HTMLBody
	}
	variables := make(map[string]string, len(request.Variables)+1)
	for key, value := range request.Variables {
		variables[key] = value
	}
	if strings.TrimSpace(variables["system_name"]) == "" {
		variables["system_name"] = systemName
	}
	rendered, err := renderTemplate(definition, subject, htmlBody, variables)
	if err != nil {
		return fmt.Errorf("render email event %s: %w", definition.Event, err)
	}
	if err := s.transport.Send(ctx, config, Message{
		To:       recipient,
		Subject:  rendered.Subject,
		HTMLBody: rendered.HTMLBody,
	}); err != nil {
		s.logger.Warn(
			"send notification email failed",
			"event", definition.Event,
			"error", redact.String(err.Error()),
		)
		return deliveryError("通知邮件发送失败", err)
	}
	return nil
}

// ListTemplates returns every supported event/locale pair with active overrides applied.
func (s *Service) ListTemplates(ctx context.Context) (TemplateListResponse, error) {
	overrides, err := s.repo.ListTemplateOverrides(ctx)
	if err != nil {
		return TemplateListResponse{}, fmt.Errorf("list email template overrides: %w", err)
	}
	overrideByKey := make(map[string]TemplateOverride, len(overrides))
	for _, override := range overrides {
		overrideByKey[templateKey(override.Event, override.Locale)] = override
	}
	items := make([]TemplateResponse, 0, len(officialTemplates))
	for _, definition := range officialTemplates {
		override, ok := overrideByKey[templateKey(string(definition.Event), definition.Locale)]
		if ok {
			items = append(items, templateResponse(definition, &override))
		} else {
			items = append(items, templateResponse(definition, nil))
		}
	}
	return TemplateListResponse{Items: items}, nil
}

// Template returns one supported event/locale pair with its active override.
func (s *Service) Template(ctx context.Context, event string, locale string) (TemplateResponse, error) {
	definition, err := normalizeTemplateIdentity(event, locale)
	if err != nil {
		return TemplateResponse{}, err
	}
	override, found, err := s.repo.GetTemplateOverride(ctx, string(definition.Event), definition.Locale)
	if err != nil {
		return TemplateResponse{}, fmt.Errorf("get email template override: %w", err)
	}
	if found {
		return templateResponse(definition, &override), nil
	}
	return templateResponse(definition, nil), nil
}

// UpdateTemplate validates and saves an administrator template override.
func (s *Service) UpdateTemplate(ctx context.Context, event string, locale string, input TemplateUpdateInput, adminID string) (TemplateResponse, error) {
	definition, err := normalizeTemplateIdentity(event, locale)
	if err != nil {
		return TemplateResponse{}, err
	}
	adminID = strings.TrimSpace(adminID)
	if adminID == "" {
		return TemplateResponse{}, badRequest("管理员 ID 不能为空")
	}
	subject, htmlBody, err := validateTemplateContent(definition, input.Subject, input.HTMLBody)
	if err != nil {
		return TemplateResponse{}, err
	}
	override := TemplateOverride{
		Event:     string(definition.Event),
		Locale:    definition.Locale,
		Subject:   subject,
		HTMLBody:  htmlBody,
		UpdatedBy: adminID,
		UpdatedAt: s.now(),
	}
	if err := s.repo.UpsertTemplate(ctx, override); err != nil {
		return TemplateResponse{}, fmt.Errorf("save email template override: %w", err)
	}
	return templateResponse(definition, &override), nil
}

// RestoreTemplate removes an override and returns the official template.
func (s *Service) RestoreTemplate(ctx context.Context, event string, locale string) (TemplateResponse, error) {
	definition, err := normalizeTemplateIdentity(event, locale)
	if err != nil {
		return TemplateResponse{}, err
	}
	if _, err := s.repo.DeleteTemplate(ctx, string(definition.Event), definition.Locale); err != nil {
		return TemplateResponse{}, fmt.Errorf("restore official email template: %w", err)
	}
	return templateResponse(definition, nil), nil
}

// PreviewTemplate renders a draft or active template with safe sample variables.
func (s *Service) PreviewTemplate(ctx context.Context, input TemplatePreviewInput) (TemplatePreviewResponse, error) {
	definition, err := normalizeTemplateIdentity(input.Event, input.Locale)
	if err != nil {
		return TemplatePreviewResponse{}, err
	}
	active, err := s.Template(ctx, string(definition.Event), definition.Locale)
	if err != nil {
		return TemplatePreviewResponse{}, err
	}
	subject, htmlBody := active.Subject, active.HTMLBody
	if input.Subject != nil {
		subject = *input.Subject
	}
	if input.HTMLBody != nil {
		htmlBody = *input.HTMLBody
	}
	subject, htmlBody, err = validateTemplateContent(definition, subject, htmlBody)
	if err != nil {
		return TemplatePreviewResponse{}, err
	}
	variables := sampleVariables(definition)
	if len(input.Variables) > len(definition.Variables) {
		return TemplatePreviewResponse{}, badRequest("variables 包含不支持的模板变量")
	}
	for key, value := range input.Variables {
		if !templateVariableAllowed(definition, key) {
			return TemplatePreviewResponse{}, badRequest("不支持的模板变量: " + key)
		}
		if len(value) > 4096 {
			return TemplatePreviewResponse{}, badRequest("模板变量长度不能超过 4096 字节")
		}
		variables[key] = value
	}
	return renderTemplate(definition, subject, htmlBody, variables)
}

// NotificationResultFromError translates a best-effort delivery result for administrator responses.
func NotificationResultFromError(err error) NotificationResult {
	if err == nil {
		return NotificationResult{Status: NotificationSent, Message: "通知邮件已发送"}
	}
	if errors.Is(err, ErrNotConfigured) {
		return NotificationResult{Status: NotificationSkipped, Message: "邮件服务未配置，未发送通知邮件"}
	}
	return NotificationResult{Status: NotificationFailed, Message: "通知邮件发送失败"}
}

func (s *Service) resolveConfig(ctx context.Context, override SMTPSettingsOverride) (SMTPConfig, string, error) {
	config, _, systemName, err := s.loadStoredConfig(ctx, true)
	if err != nil {
		return SMTPConfig{}, "", err
	}
	if override.SMTPHost != nil {
		config.Host = strings.TrimSpace(*override.SMTPHost)
	}
	if override.SMTPPort != nil {
		config.Port = *override.SMTPPort
	}
	if override.SMTPUsername != nil {
		config.Username = strings.TrimSpace(*override.SMTPUsername)
	}
	if override.SMTPPassword != nil && *override.SMTPPassword != "" {
		if len(*override.SMTPPassword) > maxPasswordSize {
			return SMTPConfig{}, "", badRequest("smtp_password 长度不能超过 4096 字节")
		}
		config.Password = *override.SMTPPassword
	}
	if override.SMTPFrom != nil {
		config.From = strings.TrimSpace(*override.SMTPFrom)
	}
	if override.SMTPFromName != nil {
		config.FromName = strings.TrimSpace(*override.SMTPFromName)
	}
	if override.SMTPUseTLS != nil {
		config.UseTLS = *override.SMTPUseTLS
	}
	return config, systemName, nil
}

func (s *Service) loadStoredConfig(ctx context.Context, decryptPassword bool) (SMTPConfig, bool, string, error) {
	values, err := s.repo.GetSettings(ctx, smtpSettingKeys)
	if err != nil {
		return SMTPConfig{}, false, "", fmt.Errorf("get smtp settings: %w", err)
	}
	port := defaultSMTPPort
	if value := strings.TrimSpace(values[settingSMTPPort]); value != "" {
		port, err = strconv.Atoi(value)
		if err != nil {
			return SMTPConfig{}, false, "", fmt.Errorf("parse stored smtp port: %w", err)
		}
	}
	passwordEncrypted := strings.TrimSpace(values[settingSMTPPassword])
	password := ""
	if decryptPassword && passwordEncrypted != "" {
		password, err = s.cipher.Decrypt(passwordEncrypted)
		if err != nil {
			return SMTPConfig{}, false, "", fmt.Errorf("decrypt smtp password: %w", err)
		}
	}
	useTLS, err := strconv.ParseBool(defaultString(values[settingSMTPUseTLS], "false"))
	if err != nil {
		return SMTPConfig{}, false, "", fmt.Errorf("parse stored smtp tls setting: %w", err)
	}
	systemName := strings.TrimSpace(values[settingSystemName])
	if systemName == "" {
		systemName = defaultSystem
	}
	return SMTPConfig{
		Host:     strings.TrimSpace(values[settingSMTPHost]),
		Port:     port,
		Username: strings.TrimSpace(values[settingSMTPUsername]),
		Password: password,
		From:     strings.TrimSpace(values[settingSMTPFrom]),
		FromName: strings.TrimSpace(values[settingSMTPFromName]),
		UseTLS:   useTLS,
	}, passwordEncrypted != "", systemName, nil
}

func settingsResponse(config SMTPConfig, passwordConfigured bool) SMTPSettingsResponse {
	return SMTPSettingsResponse{
		SMTPHost:               config.Host,
		SMTPPort:               config.Port,
		SMTPUsername:           config.Username,
		SMTPFrom:               config.From,
		SMTPFromName:           config.FromName,
		SMTPUseTLS:             config.UseTLS,
		SMTPPasswordConfigured: passwordConfigured,
		Configured:             config.Host != "" && config.From != "" && config.Port > 0 && config.Port <= 65535,
	}
}

func validateConnectionConfig(config SMTPConfig) error {
	if err := validateSMTPHost(config.Host); err != nil {
		return err
	}
	if config.Port < 1 || config.Port > 65535 {
		return badRequest("smtp_port 必须在 1 到 65535 之间")
	}
	if len(config.Username) > 320 {
		return badRequest("smtp_username 长度不能超过 320")
	}
	if strings.ContainsAny(config.Username, "\r\n") {
		return badRequest("smtp_username 不能包含换行符")
	}
	if len(config.Password) > maxPasswordSize {
		return badRequest("smtp_password 长度不能超过 4096 字节")
	}
	if config.Username == "" && config.Password != "" {
		return badRequest("smtp_username 不能为空")
	}
	return nil
}

func validateDeliveryConfig(config SMTPConfig) error {
	if err := validateConnectionConfig(config); err != nil {
		return err
	}
	from, err := normalizeEmail(config.From, "smtp_from")
	if err != nil {
		return err
	}
	if from == "" {
		return badRequest("smtp_from 不能为空")
	}
	if len([]rune(config.FromName)) > 50 {
		return badRequest("smtp_from_name 长度不能超过 50")
	}
	if strings.ContainsAny(config.FromName, "\r\n") {
		return badRequest("smtp_from_name 不能包含换行符")
	}
	return nil
}

func validateSMTPHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return badRequest("smtp_host 不能为空")
	}
	if len(host) > 253 || strings.ContainsAny(host, "/\\?#@") {
		return badRequest("smtp_host 格式无效")
	}
	if strings.IndexFunc(host, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return badRequest("smtp_host 格式无效")
	}
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return badRequest("smtp_host 不能包含端口或协议")
	}
	return nil
}

func normalizeEmail(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", badRequest(field + " 不能为空")
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", badRequest(field + " 格式无效")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || !strings.EqualFold(address.Address, value) {
		return "", badRequest(field + " 格式无效")
	}
	return address.Address, nil
}

func templateVariableAllowed(definition templateDefinition, variable string) bool {
	for _, allowed := range definition.Variables {
		if variable == allowed {
			return true
		}
	}
	return false
}

func templateKey(event string, locale string) string {
	return event + "\x00" + locale
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}

func notConfigured(message string) error {
	return Error{Kind: ErrNotConfigured, Message: message}
}

func deliveryError(message string, cause error) error {
	return Error{Kind: ErrDelivery, Message: message, Cause: cause}
}
