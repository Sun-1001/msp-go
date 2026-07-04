package adminsettingshttp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	adminsettingsapp "mathstudy/backend-go/internal/application/adminsettings"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

const maxImportBytes = 100 << 20

// Service is the admin settings application surface used by HTTP handlers.
type Service interface {
	RegistrationSettings(context.Context) (adminsettingsapp.RegistrationSettingsResponse, error)
	UpdateRegistrationSettings(context.Context, bool, bool) (adminsettingsapp.RegistrationSettingsResponse, error)
	GeneralSettings(context.Context) (adminsettingsapp.GeneralSettingsResponse, error)
	UpdateGeneralSettings(context.Context, string, string) (adminsettingsapp.GeneralSettingsResponse, error)
	ExportableTables(context.Context) (adminsettingsapp.ExportableTablesResponse, error)
	ExportData(context.Context, []string, string) (adminsettingsapp.DataExportResponse, error)
	ImportData(context.Context, []byte, string) (adminsettingsapp.DataImportResponse, error)
	DatabaseMonitor(context.Context) (adminsettingsapp.DatabaseMonitorResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /admin/settings endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an admin settings HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("admin settings service is nil")
	}
	if auth == nil {
		return nil, errors.New("admin settings authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches admin settings routes under prefix, for example /api/v1/admin/settings.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/registration", h.getRegistration)
	mux.HandleFunc("PUT "+prefix+"/registration", h.updateRegistration)
	mux.HandleFunc("GET "+prefix+"/general", h.getGeneral)
	mux.HandleFunc("PUT "+prefix+"/general", h.updateGeneral)
	mux.HandleFunc("GET "+prefix+"/database/exportable-tables", h.exportableTables)
	mux.HandleFunc("POST "+prefix+"/database/export", h.exportDatabase)
	mux.HandleFunc("POST "+prefix+"/database/import", h.importDatabase)
	mux.HandleFunc("GET "+prefix+"/database/monitor", h.databaseMonitor)
}

type registrationRequest struct {
	AllowStudent bool `json:"allow_student"`
	AllowTeacher bool `json:"allow_teacher"`
}

type generalRequest struct {
	SystemName        string `json:"system_name"`
	SystemDescription string `json:"system_description"`
}

type exportRequest struct {
	Tables []string `json:"tables"`
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) getRegistration(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.RegistrationSettings(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取注册配置失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) updateRegistration(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request registrationRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.UpdateRegistrationSettings(r.Context(), request.AllowStudent, request.AllowTeacher)
	if err != nil {
		h.writeServiceError(w, err, "更新注册配置失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) getGeneral(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.GeneralSettings(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取系统基本信息失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) updateGeneral(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request generalRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.UpdateGeneralSettings(r.Context(), request.SystemName, request.SystemDescription)
	if err != nil {
		h.writeServiceError(w, err, "更新系统基本信息失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) exportableTables(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.ExportableTables(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取可导出表列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) exportDatabase(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	var request exportRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.ExportData(r.Context(), request.Tables, principal.UserID)
	if err != nil {
		h.writeServiceError(w, err, "导出数据库数据失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) importDatabase(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	if err := r.ParseMultipartForm(maxImportBytes); err != nil {
		if isRequestTooLarge(err) {
			writeAdminSettingsError(w, http.StatusBadRequest, "BAD_REQUEST", "文件大小不能超过 100MB")
			return
		}
		writeAdminSettingsError(w, http.StatusBadRequest, "BAD_REQUEST", "文件读取失败: "+redact.String(err.Error()))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAdminSettingsError(w, http.StatusBadRequest, "BAD_REQUEST", "请上传 JSON 格式的备份文件")
		return
	}
	defer file.Close()
	if header == nil || !strings.HasSuffix(strings.ToLower(header.Filename), ".json") {
		writeAdminSettingsError(w, http.StatusBadRequest, "BAD_REQUEST", "请上传 JSON 格式的备份文件")
		return
	}
	content, err := io.ReadAll(file)
	if err != nil {
		if isRequestTooLarge(err) {
			writeAdminSettingsError(w, http.StatusBadRequest, "BAD_REQUEST", "文件大小不能超过 100MB")
			return
		}
		writeAdminSettingsError(w, http.StatusBadRequest, "BAD_REQUEST", "文件读取失败: "+redact.String(err.Error()))
		return
	}
	response, err := h.service.ImportData(r.Context(), content, principal.UserID)
	if err != nil {
		h.writeServiceError(w, err, "导入数据库数据失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func isRequestTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func (h *Handler) databaseMonitor(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.DatabaseMonitor(r.Context())
	if err != nil {
		h.writeServiceError(w, err, "获取数据库监控数据失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	token, ok := httpauth.BearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAdminSettingsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(token)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAdminSettingsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	if !authapp.IsAdmin(principal) {
		writeAdminSettingsError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要管理员权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, adminsettingsapp.ErrBadRequest):
		writeAdminSettingsError(w, http.StatusBadRequest, "BAD_REQUEST", redact.String(err.Error()))
	default:
		h.logger.Error("admin settings request failed", "error", redact.String(err.Error()))
		writeAdminSettingsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := httpjson.DecodeStrict(w, r, 1<<20, target); err != nil {
		writeAdminSettingsError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体格式错误")
		return false
	}
	return true
}

func writeAdminSettingsError(w http.ResponseWriter, status int, code, message string) {
	httpjson.Write(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
