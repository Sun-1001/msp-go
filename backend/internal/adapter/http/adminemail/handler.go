package adminemailhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authapp "mathstudy/backend/internal/application/auth"
	emailapp "mathstudy/backend/internal/application/email"
	"mathstudy/backend/internal/platform/httpauth"
	"mathstudy/backend/internal/platform/httpjson"
	"mathstudy/backend/internal/platform/redact"
)

// Service is the administrator email application surface used by HTTP handlers.
type Service interface {
	Settings(context.Context) (emailapp.SMTPSettingsResponse, error)
	UpdateSettings(context.Context, emailapp.UpdateSMTPSettingsInput) (emailapp.SMTPSettingsResponse, error)
	TestSMTP(context.Context, emailapp.SMTPSettingsOverride) (emailapp.ActionResponse, error)
	SendTestEmail(context.Context, emailapp.SendTestEmailInput) (emailapp.ActionResponse, error)
	ListTemplates(context.Context) (emailapp.TemplateListResponse, error)
	Template(context.Context, string, string) (emailapp.TemplateResponse, error)
	UpdateTemplate(context.Context, string, string, emailapp.TemplateUpdateInput, string) (emailapp.TemplateResponse, error)
	RestoreTemplate(context.Context, string, string) (emailapp.TemplateResponse, error)
	PreviewTemplate(context.Context, emailapp.TemplatePreviewInput) (emailapp.TemplatePreviewResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves administrator SMTP and email-template endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an administrator email handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("admin email service is nil")
	}
	if auth == nil {
		return nil, errors.New("admin email authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches email routes under /admin/settings.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/email", h.getSettings)
	mux.HandleFunc("PUT "+prefix+"/email", h.updateSettings)
	mux.HandleFunc("POST "+prefix+"/test-smtp", h.testSMTP)
	mux.HandleFunc("POST "+prefix+"/send-test-email", h.sendTestEmail)
	mux.HandleFunc("GET "+prefix+"/email-templates", h.listTemplates)
	mux.HandleFunc("GET "+prefix+"/email-templates/{event}/{locale}", h.getTemplate)
	mux.HandleFunc("PUT "+prefix+"/email-templates/{event}/{locale}", h.updateTemplate)
	mux.HandleFunc("POST "+prefix+"/email-templates/{event}/{locale}/restore", h.restoreTemplate)
	mux.HandleFunc("POST "+prefix+"/email-template-preview", h.previewTemplate)
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.Settings(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取邮件配置失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request emailapp.UpdateSMTPSettingsInput
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.UpdateSettings(r.Context(), request)
	if err != nil {
		h.writeServiceError(w, err, "更新邮件配置失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) testSMTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request emailapp.SMTPSettingsOverride
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.TestSMTP(r.Context(), request)
	if err != nil {
		h.writeServiceError(w, err, "SMTP 连接测试失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) sendTestEmail(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request emailapp.SendTestEmailInput
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.SendTestEmail(r.Context(), request)
	if err != nil {
		h.writeServiceError(w, err, "测试邮件发送失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.ListTemplates(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取邮件模板失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) getTemplate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.Template(r.Context(), r.PathValue("event"), r.PathValue("locale"))
	if err != nil {
		h.writeServiceError(w, err, "获取邮件模板失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) updateTemplate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	var request emailapp.TemplateUpdateInput
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.UpdateTemplate(
		r.Context(),
		r.PathValue("event"),
		r.PathValue("locale"),
		request,
		principal.UserID,
	)
	if err != nil {
		h.writeServiceError(w, err, "更新邮件模板失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) restoreTemplate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.RestoreTemplate(r.Context(), r.PathValue("event"), r.PathValue("locale"))
	if err != nil {
		h.writeServiceError(w, err, "恢复官方邮件模板失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) previewTemplate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request emailapp.TemplatePreviewInput
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.PreviewTemplate(r.Context(), request)
	if err != nil {
		h.writeServiceError(w, err, "预览邮件模板失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsAdmin,
		"权限不足，需要管理员权限", writeAdminEmailError,
	)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, emailapp.ErrBadRequest):
		writeAdminEmailError(w, http.StatusBadRequest, "BAD_REQUEST", redact.String(err.Error()))
	case errors.Is(err, emailapp.ErrNotConfigured):
		writeAdminEmailError(w, http.StatusUnprocessableEntity, "EMAIL_NOT_CONFIGURED", redact.String(err.Error()))
	case errors.Is(err, emailapp.ErrDelivery):
		h.logger.Warn("administrator email delivery failed", "error", loggableError(err))
		writeAdminEmailError(w, http.StatusUnprocessableEntity, "EMAIL_DELIVERY_FAILED", redact.String(err.Error()))
	default:
		h.logger.Error("administrator email request failed", "error", loggableError(err))
		writeAdminEmailError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func loggableError(err error) string {
	var appError emailapp.Error
	if errors.As(err, &appError) && appError.Cause != nil {
		return redact.String(appError.Cause.Error())
	}
	return redact.String(err.Error())
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	return httpjson.DecodeStrictOrDetailError(w, r, 1<<20, target)
}

func writeAdminEmailError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
