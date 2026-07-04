package adminuserhttp

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	adminuserapp "mathstudy/backend-go/internal/application/adminuser"
	authapp "mathstudy/backend-go/internal/application/auth"
	"mathstudy/backend-go/internal/platform/csvsafe"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/redact"
)

const (
	maxImportBytes      = 5 << 20
	maxImportRows       = 5000
	maxImportColumns    = 20
	maxImportFieldRunes = 1000
)

// Service is the admin user application surface used by HTTP handlers.
type Service interface {
	AccountStats(context.Context) (adminuserapp.AccountStats, error)
	ListUsers(context.Context, adminuserapp.ListFilter) (adminuserapp.ListResponse, error)
	UpdateUserStatus(context.Context, string, string) (adminuserapp.UpdateResponse, error)
	UpdateUser(context.Context, string, adminuserapp.Update) (adminuserapp.UpdateResponse, error)
	DeleteUser(context.Context, string) (adminuserapp.DeleteResponse, error)
	CreateUser(context.Context, adminuserapp.Create) (adminuserapp.CreateResponse, error)
	ExportUsers(context.Context, adminuserapp.ListFilter) ([]adminuserapp.ExportUser, error)
	ImportUsers(context.Context, []adminuserapp.ImportUser) (adminuserapp.ImportResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /admin/users endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an admin user HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("admin user service is nil")
	}
	if auth == nil {
		return nil, errors.New("admin user authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches admin user routes under prefix, for example /api/v1/admin/users.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/stats", h.stats)
	mux.HandleFunc("GET "+prefix+"/export", h.exportUsers)
	mux.HandleFunc("POST "+prefix+"/import", h.importUsers)
	mux.HandleFunc("GET "+prefix, h.listUsers)
	mux.HandleFunc("POST "+prefix, h.createUser)
	mux.HandleFunc("PATCH "+prefix+"/{user_id}/status", h.updateUserStatus)
	mux.HandleFunc("PUT "+prefix+"/{user_id}", h.updateUser)
	mux.HandleFunc("DELETE "+prefix+"/{user_id}", h.deleteUser)
}

type createRequest struct {
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	Role        string  `json:"role"`
	DisplayName *string `json:"display_name"`
}

type updateRequest struct {
	DisplayName *string `json:"display_name"`
	Password    *string `json:"password"`
}

type statusUpdateRequest struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.AccountStats(r.Context())
	if err != nil {
		h.logger.Error("get admin user stats failed", "error", redact.String(err.Error()))
		writeAdminUserError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取账户统计失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	filter, ok := parseListFilter(w, r)
	if !ok {
		return
	}
	response, err := h.service.ListUsers(r.Context(), filter)
	if err != nil {
		h.writeServiceError(w, err, "获取用户列表失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request createRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.CreateUser(r.Context(), adminuserapp.Create{
		Username:    request.Username,
		Email:       request.Email,
		Password:    request.Password,
		Role:        request.Role,
		DisplayName: request.DisplayName,
	})
	if err != nil {
		h.writeServiceError(w, err, "创建用户失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request statusUpdateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.UpdateUserStatus(r.Context(), r.PathValue("user_id"), request.Status)
	if err != nil {
		h.writeServiceError(w, err, "更新用户状态失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request updateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	response, err := h.service.UpdateUser(r.Context(), r.PathValue("user_id"), adminuserapp.Update{
		DisplayName: request.DisplayName,
		Password:    request.Password,
	})
	if err != nil {
		h.writeServiceError(w, err, "更新用户信息失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.DeleteUser(r.Context(), r.PathValue("user_id"))
	if err != nil {
		h.writeServiceError(w, err, "删除用户失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) exportUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	filter, ok := parseExportFilter(w, r)
	if !ok {
		return
	}
	users, err := h.service.ExportUsers(r.Context(), filter)
	if err != nil {
		h.writeServiceError(w, err, "导出用户失败")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=users_export.csv")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"用户名", "邮箱", "显示名称", "角色", "状态", "创建时间"})
	for _, account := range users {
		_ = writer.Write(csvsafe.Row(
			account.Username,
			account.Email,
			account.DisplayName,
			account.Role,
			account.Status,
			account.CreatedAt.Format("2006-01-02 15:04:05"),
		))
	}
	writer.Flush()
}

func (h *Handler) importUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	if err := r.ParseMultipartForm(maxImportBytes); err != nil {
		writeAdminUserError(w, http.StatusBadRequest, "BAD_REQUEST", "文件读取失败: "+redact.String(err.Error()))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAdminUserError(w, http.StatusBadRequest, "BAD_REQUEST", "请上传 CSV 格式的文件")
		return
	}
	defer file.Close()
	if header == nil || !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		writeAdminUserError(w, http.StatusBadRequest, "BAD_REQUEST", "请上传 CSV 格式的文件")
		return
	}
	content, err := io.ReadAll(file)
	if err != nil {
		writeAdminUserError(w, http.StatusBadRequest, "BAD_REQUEST", "文件读取失败: "+redact.String(err.Error()))
		return
	}
	text, err := decodeCSVContent(content)
	if err != nil {
		writeAdminUserError(w, http.StatusBadRequest, "BAD_REQUEST", "文件读取失败: "+redact.String(err.Error()))
		return
	}
	users, err := parseImportCSV(text)
	if err != nil {
		writeAdminUserError(w, http.StatusBadRequest, "BAD_REQUEST", "CSV 解析失败: "+redact.String(err.Error()))
		return
	}
	if len(users) == 0 {
		writeAdminUserError(w, http.StatusBadRequest, "BAD_REQUEST", "CSV 文件为空或格式不正确")
		return
	}
	response, err := h.service.ImportUsers(r.Context(), users)
	if err != nil {
		h.writeServiceError(w, err, "导入用户失败")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	token, ok := httpauth.BearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAdminUserError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(token)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeAdminUserError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	if !authapp.IsAdmin(principal) {
		writeAdminUserError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要管理员权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, adminuserapp.ErrBadRequest):
		writeAdminUserError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", redact.String(err.Error()))
	case errors.Is(err, adminuserapp.ErrNotFound):
		writeAdminUserError(w, http.StatusNotFound, "NOT_FOUND", redact.String(err.Error()))
	default:
		h.logger.Error("admin user request failed", "error", redact.String(err.Error()))
		writeAdminUserError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
	}
}

func parseListFilter(w http.ResponseWriter, r *http.Request) (adminuserapp.ListFilter, bool) {
	page, ok := parseIntQuery(w, r.URL.Query().Get("page"), 1, "page")
	if !ok {
		return adminuserapp.ListFilter{}, false
	}
	pageSize, ok := parseIntQuery(w, r.URL.Query().Get("page_size"), 10, "page_size")
	if !ok {
		return adminuserapp.ListFilter{}, false
	}
	if page < 1 {
		writeAdminUserError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page 必须大于等于 1")
		return adminuserapp.ListFilter{}, false
	}
	if pageSize < 1 || pageSize > 100 {
		writeAdminUserError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "page_size 必须在 1 到 100 之间")
		return adminuserapp.ListFilter{}, false
	}
	filter, ok := parseExportFilter(w, r)
	if !ok {
		return adminuserapp.ListFilter{}, false
	}
	filter.Page = page
	filter.PageSize = pageSize
	return filter, true
}

func parseExportFilter(w http.ResponseWriter, r *http.Request) (adminuserapp.ListFilter, bool) {
	query := r.URL.Query()
	filter := adminuserapp.ListFilter{
		Search: query.Get("search"),
		Role:   query.Get("role"),
		Status: query.Get("status"),
	}
	return filter, true
}

func parseIntQuery(w http.ResponseWriter, value string, fallback int, name string) (int, bool) {
	parsed, err := httpquery.Int(value, fallback)
	if err != nil {
		writeAdminUserError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 必须是整数")
		return 0, false
	}
	return parsed, true
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := httpjson.DecodeStrict(w, r, 1<<20, target); err != nil {
		writeAdminUserError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "请求体格式错误")
		return false
	}
	return true
}

func decodeCSVContent(content []byte) (string, error) {
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(content) {
		return string(content), nil
	}
	reader := transform.NewReader(bytes.NewReader(content), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func parseImportCSV(text string) ([]adminuserapp.ImportUser, error) {
	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if len(header) > maxImportColumns {
		return nil, errors.New("CSV 列数超过限制")
	}
	if err := validateImportRecord(header); err != nil {
		return nil, err
	}
	fields := make([]string, len(header))
	for index, name := range header {
		fields[index] = importFieldName(name)
	}

	users := []adminuserapp.ImportUser{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) > maxImportColumns {
			return nil, errors.New("CSV 列数超过限制")
		}
		if err := validateImportRecord(record); err != nil {
			return nil, err
		}
		var row adminuserapp.ImportUser
		hasKnownField := false
		for index, field := range fields {
			if field == "" || index >= len(record) {
				continue
			}
			hasKnownField = true
			value := record[index]
			switch field {
			case "username":
				row.Username = value
			case "email":
				row.Email = value
			case "password":
				row.Password = value
			case "role":
				row.Role = value
			case "display_name":
				row.DisplayName = &value
			}
		}
		if hasKnownField {
			users = append(users, row)
			if len(users) > maxImportRows {
				return nil, errors.New("CSV 行数超过限制")
			}
		}
	}
	return users, nil
}

func validateImportRecord(record []string) error {
	for _, value := range record {
		if len([]rune(value)) > maxImportFieldRunes {
			return errors.New("CSV 字段长度超过限制")
		}
	}
	return nil
}

func importFieldName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "用户名", "username":
		return "username"
	case "邮箱", "email":
		return "email"
	case "密码", "password":
		return "password"
	case "角色", "role":
		return "role"
	case "显示名称", "display_name":
		return "display_name"
	default:
		return ""
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAdminUserError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
