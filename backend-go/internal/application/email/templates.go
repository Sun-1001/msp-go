package email

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

const (
	maxTemplateSubjectRunes = 200
	maxTemplateHTMLBytes    = 100 << 10
	maxRenderedHTMLBytes    = 200 << 10
)

type templateDefinition struct {
	Event     Event
	Locale    string
	Name      string
	Subject   string
	HTMLBody  string
	Variables []string
}

var officialTemplates = []templateDefinition{
	{
		Event:     EventPasswordReset,
		Locale:    LocaleZhCN,
		Name:      "密码恢复",
		Subject:   "{{.system_name}} 密码恢复通知",
		Variables: []string{"system_name", "display_name", "username", "temp_password"},
		HTMLBody: emailDocument(`
<h1>密码恢复申请已通过</h1>
<p>{{.display_name}}，您好：</p>
<p>管理员已通过账号 <strong>{{.username}}</strong> 的密码恢复申请。</p>
<div class="code">{{.temp_password}}</div>
<p>请使用该临时密码登录，并立即修改密码。若您未提交申请，请联系管理员。</p>`),
	},
	{
		Event:     EventPasswordReset,
		Locale:    LocaleEnUS,
		Name:      "Password recovery",
		Subject:   "Password recovery for {{.system_name}}",
		Variables: []string{"system_name", "display_name", "username", "temp_password"},
		HTMLBody: emailDocument(`
<h1>Your password recovery request was approved</h1>
<p>Hello {{.display_name}},</p>
<p>An administrator approved the password recovery request for <strong>{{.username}}</strong>.</p>
<div class="code">{{.temp_password}}</div>
<p>Sign in with this temporary password and change it immediately. Contact an administrator if you did not request this.</p>`),
	},
	{
		Event:     EventAccountSuspended,
		Locale:    LocaleZhCN,
		Name:      "账号停用",
		Subject:   "{{.system_name}} 账号停用通知",
		Variables: []string{"system_name", "display_name", "username", "role_name"},
		HTMLBody: emailDocument(`
<h1>账号已停用</h1>
<p>{{.display_name}}，您好：</p>
<p>您的{{.role_name}}账号 <strong>{{.username}}</strong> 已由管理员停用，当前无法登录。</p>
<p>如需了解详情或申请恢复，请联系管理员。</p>`),
	},
	{
		Event:     EventAccountSuspended,
		Locale:    LocaleEnUS,
		Name:      "Account suspended",
		Subject:   "Your {{.system_name}} account was suspended",
		Variables: []string{"system_name", "display_name", "username", "role_name"},
		HTMLBody: emailDocument(`
<h1>Account suspended</h1>
<p>Hello {{.display_name}},</p>
<p>Your {{.role_name}} account <strong>{{.username}}</strong> was suspended by an administrator and cannot currently sign in.</p>
<p>Contact an administrator for details or to request reactivation.</p>`),
	},
	{
		Event:     EventAccountDeactivated,
		Locale:    LocaleZhCN,
		Name:      "账号注销",
		Subject:   "{{.system_name}} 账号注销通知",
		Variables: []string{"system_name", "display_name", "username", "role_name"},
		HTMLBody: emailDocument(`
<h1>账号已注销</h1>
<p>{{.display_name}}，您好：</p>
<p>您的{{.role_name}}账号 <strong>{{.username}}</strong> 已被注销。</p>
<p>如对此操作有疑问，请联系管理员。</p>`),
	},
	{
		Event:     EventAccountDeactivated,
		Locale:    LocaleEnUS,
		Name:      "Account deactivated",
		Subject:   "Your {{.system_name}} account was deactivated",
		Variables: []string{"system_name", "display_name", "username", "role_name"},
		HTMLBody: emailDocument(`
<h1>Account deactivated</h1>
<p>Hello {{.display_name}},</p>
<p>Your {{.role_name}} account <strong>{{.username}}</strong> was deactivated.</p>
<p>Contact an administrator if you have questions about this action.</p>`),
	},
	{
		Event:     EventAccountReactivated,
		Locale:    LocaleZhCN,
		Name:      "账号恢复",
		Subject:   "{{.system_name}} 账号恢复通知",
		Variables: []string{"system_name", "display_name", "username", "role_name"},
		HTMLBody: emailDocument(`
<h1>账号已恢复</h1>
<p>{{.display_name}}，您好：</p>
<p>您的{{.role_name}}账号 <strong>{{.username}}</strong> 已恢复，可以重新登录。</p>`),
	},
	{
		Event:     EventAccountReactivated,
		Locale:    LocaleEnUS,
		Name:      "Account reactivated",
		Subject:   "Your {{.system_name}} account was reactivated",
		Variables: []string{"system_name", "display_name", "username", "role_name"},
		HTMLBody: emailDocument(`
<h1>Account reactivated</h1>
<p>Hello {{.display_name}},</p>
<p>Your {{.role_name}} account <strong>{{.username}}</strong> was reactivated and can sign in again.</p>`),
	},
}

func emailDocument(content string) string {
	return `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;background:#f4f6f8;color:#17202a;font-family:Arial,'Microsoft YaHei',sans-serif">
<div style="max-width:640px;margin:0 auto;padding:32px 16px">
<div style="background:#ffffff;border:1px solid #dfe4e8;border-radius:8px;padding:32px">
<style>h1{font-size:22px;margin:0 0 24px}p{font-size:15px;line-height:1.7;margin:12px 0}.code{margin:20px 0;padding:16px;border:1px solid #b8c2cc;border-radius:6px;background:#f7f9fa;font-family:Consolas,monospace;font-size:20px;font-weight:700;letter-spacing:1px;text-align:center}</style>` +
		strings.TrimSpace(content) +
		`</div></div></body></html>`
}

func findTemplateDefinition(event string, locale string) (templateDefinition, bool) {
	for _, definition := range officialTemplates {
		if string(definition.Event) == event && definition.Locale == locale {
			return definition, true
		}
	}
	return templateDefinition{}, false
}

func normalizeTemplateIdentity(event string, locale string) (templateDefinition, error) {
	event = strings.ToLower(strings.TrimSpace(event))
	locale = normalizeLocale(locale)
	definition, ok := findTemplateDefinition(event, locale)
	if !ok {
		return templateDefinition{}, badRequest("不支持的邮件模板事件或语言")
	}
	return definition, nil
}

func normalizeLocale(locale string) string {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "", "zh", "zh-cn", "zh_hans", "zh-hans":
		return LocaleZhCN
	case "en", "en-us", "en_us":
		return LocaleEnUS
	default:
		return strings.TrimSpace(locale)
	}
}

func validateTemplateContent(definition templateDefinition, subject string, htmlBody string) (string, string, error) {
	subject = strings.TrimSpace(subject)
	htmlBody = strings.TrimSpace(htmlBody)
	if subject == "" || len([]rune(subject)) > maxTemplateSubjectRunes {
		return "", "", badRequest("subject 长度必须在 1 到 200 之间")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return "", "", badRequest("subject 不能包含换行符")
	}
	if htmlBody == "" || len(htmlBody) > maxTemplateHTMLBytes {
		return "", "", badRequest("html_body 长度必须在 1 到 102400 字节之间")
	}
	if _, err := renderTemplate(definition, subject, htmlBody, sampleVariables(definition)); err != nil {
		return "", "", badRequest("模板语法或变量无效: " + err.Error())
	}
	return subject, htmlBody, nil
}

func renderTemplate(definition templateDefinition, subject string, htmlBody string, variables map[string]string) (TemplatePreviewResponse, error) {
	if err := validateRequiredVariables(definition, variables); err != nil {
		return TemplatePreviewResponse{}, err
	}
	subjectTemplate, err := texttemplate.New("subject").Option("missingkey=error").Parse(subject)
	if err != nil {
		return TemplatePreviewResponse{}, fmt.Errorf("parse subject: %w", err)
	}
	var renderedSubject bytes.Buffer
	if err := subjectTemplate.Execute(&renderedSubject, variables); err != nil {
		return TemplatePreviewResponse{}, fmt.Errorf("render subject: %w", err)
	}
	if strings.ContainsAny(renderedSubject.String(), "\r\n") {
		return TemplatePreviewResponse{}, fmt.Errorf("rendered subject contains a line break")
	}

	bodyTemplate, err := htmltemplate.New("body").Option("missingkey=error").Parse(htmlBody)
	if err != nil {
		return TemplatePreviewResponse{}, fmt.Errorf("parse html body: %w", err)
	}
	var renderedBody bytes.Buffer
	if err := bodyTemplate.Execute(&renderedBody, variables); err != nil {
		return TemplatePreviewResponse{}, fmt.Errorf("render html body: %w", err)
	}
	if renderedBody.Len() > maxRenderedHTMLBytes {
		return TemplatePreviewResponse{}, fmt.Errorf("rendered html body exceeds size limit")
	}
	return TemplatePreviewResponse{Subject: renderedSubject.String(), HTMLBody: renderedBody.String()}, nil
}

func validateRequiredVariables(definition templateDefinition, variables map[string]string) error {
	for _, name := range definition.Variables {
		if _, ok := variables[name]; !ok {
			return fmt.Errorf("missing variable %s", name)
		}
	}
	return nil
}

func sampleVariables(definition templateDefinition) map[string]string {
	samples := map[string]string{
		"system_name":   "高等数学智能学习平台",
		"display_name":  "示例用户",
		"username":      "student001",
		"temp_password": "Temp#2026Pass",
		"role_name":     "学生",
	}
	variables := make(map[string]string, len(definition.Variables))
	for _, name := range definition.Variables {
		variables[name] = samples[name]
	}
	return variables
}

func templateResponse(definition templateDefinition, override *TemplateOverride) TemplateResponse {
	response := TemplateResponse{
		Event:     string(definition.Event),
		Locale:    definition.Locale,
		Name:      definition.Name,
		Subject:   definition.Subject,
		HTMLBody:  definition.HTMLBody,
		Variables: append([]string(nil), definition.Variables...),
	}
	if override != nil {
		updatedAt := override.UpdatedAt
		response.Subject = override.Subject
		response.HTMLBody = override.HTMLBody
		response.IsCustom = true
		response.UpdatedAt = &updatedAt
	}
	return response
}
