package classroomhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	classroomapp "mathstudy/backend-go/internal/application/classroom"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the classroom application surface used by HTTP handlers.
type Service interface {
	CreateClass(context.Context, string, string, *string) (classroomapp.ClassCreateResponse, error)
	ListTeacherClasses(context.Context, string) (classroomapp.ClassListResponse, error)
	GetTeacherClassDetail(context.Context, string, string) (classroomapp.ClassDetailResponse, error)
	RemoveStudent(context.Context, string, string, string) (classroomapp.ActionResponse, error)
	DisbandClass(context.Context, string, string) (classroomapp.ActionResponse, error)
	LookupClass(context.Context, string) (classroomapp.ClassLookupResponse, error)
	JoinClass(context.Context, string, string) (classroomapp.JoinClassResponse, error)
	LeaveClass(context.Context, string) (classroomapp.ActionResponse, error)
	GetStudentClass(context.Context, string) (classroomapp.StudentClassResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /classes endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a classroom HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("classroom service is nil")
	}
	if auth == nil {
		return nil, errors.New("classroom authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches class routes under prefix, for example /api/v1/classes.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("POST "+prefix, h.create)
	mux.HandleFunc("GET "+prefix+"/teacher", h.listTeacherClasses)
	mux.HandleFunc("GET "+prefix+"/teacher/{class_id}", h.teacherClassDetail)
	mux.HandleFunc("DELETE "+prefix+"/teacher/{class_id}/students/{student_id}", h.removeStudent)
	mux.HandleFunc("DELETE "+prefix+"/teacher/{class_id}", h.disbandClass)
	mux.HandleFunc("GET "+prefix+"/lookup", h.lookupClass)
	mux.HandleFunc("POST "+prefix+"/join", h.joinClass)
	mux.HandleFunc("POST "+prefix+"/leave", h.leaveClass)
	mux.HandleFunc("GET "+prefix+"/me", h.myClass)
}

type createRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type joinRequest struct {
	Code string `json:"code"`
}

const maxJSONBodyBytes = 1 << 20

type errorResponse struct {
	Detail  string `json:"detail"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	var request createRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	if !validateClassName(w, request.Name) || !validateOptionalLength(w, request.Description, 500, "description") {
		return
	}
	response, err := h.service.CreateClass(r.Context(), principal.UserID, request.Name, request.Description)
	if err != nil {
		if errors.Is(err, classroomapp.ErrForbidden) {
			writeClassError(w, http.StatusForbidden, "FORBIDDEN", "非教师账号，无法创建班级")
			return
		}
		if errors.Is(err, classroomapp.ErrConflict) {
			writeClassError(w, http.StatusConflict, "CONFLICT", "班级号生成冲突，请稍后重试")
			return
		}
		h.logClassroomError("create class failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "创建班级失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) listTeacherClasses(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	response, err := h.service.ListTeacherClasses(r.Context(), principal.UserID)
	if err != nil {
		h.logClassroomError("list teacher classes failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取班级列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) teacherClassDetail(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetTeacherClassDetail(r.Context(), principal.UserID, r.PathValue("class_id"))
	if err != nil {
		if errors.Is(err, classroomapp.ErrNotFound) {
			writeClassError(w, http.StatusNotFound, "NOT_FOUND", "班级不存在或无权限访问")
			return
		}
		h.logClassroomError("get teacher class detail failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取班级详情失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) removeStudent(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	response, err := h.service.RemoveStudent(r.Context(), principal.UserID, r.PathValue("class_id"), r.PathValue("student_id"))
	if err != nil {
		if errors.Is(err, classroomapp.ErrNotFound) {
			writeClassError(w, http.StatusNotFound, "NOT_FOUND", "班级或学生不存在")
			return
		}
		h.logClassroomError("remove class student failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "移除学生失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) disbandClass(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireTeacher(w, r)
	if !ok {
		return
	}
	response, err := h.service.DisbandClass(r.Context(), principal.UserID, r.PathValue("class_id"))
	if err != nil {
		if errors.Is(err, classroomapp.ErrNotFound) {
			writeClassError(w, http.StatusNotFound, "NOT_FOUND", "班级不存在或无权限操作")
			return
		}
		h.logClassroomError("disband class failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "解散班级失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) lookupClass(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePrincipal(w, r); !ok {
		return
	}
	code := r.URL.Query().Get("code")
	if !validateClassCode(w, code) {
		return
	}
	response, err := h.service.LookupClass(r.Context(), code)
	if err != nil {
		h.logClassroomError("lookup class failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "查询班级失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) joinClass(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	var request joinRequest
	if !decodeRequest(w, r, &request) || !validateClassCode(w, request.Code) {
		return
	}
	response, err := h.service.JoinClass(r.Context(), principal.UserID, request.Code)
	if err != nil {
		if errors.Is(err, classroomapp.ErrNotFound) {
			writeClassError(w, http.StatusNotFound, "NOT_FOUND", "班级号不存在")
			return
		}
		if errors.Is(err, classroomapp.ErrForbidden) {
			writeClassError(w, http.StatusForbidden, "FORBIDDEN", "非学生账号，无法加入班级")
			return
		}
		if errors.Is(err, classroomapp.ErrConflict) {
			writeClassError(w, http.StatusConflict, "CONFLICT", "当前已加入班级，请先退出后再加入")
			return
		}
		h.logClassroomError("join class failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "加入班级失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) leaveClass(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	response, err := h.service.LeaveClass(r.Context(), principal.UserID)
	if err != nil {
		if errors.Is(err, classroomapp.ErrNotFound) {
			writeClassError(w, http.StatusNotFound, "NOT_FOUND", "未加入任何班级")
			return
		}
		h.logClassroomError("leave class failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "退出班级失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) myClass(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetStudentClass(r.Context(), principal.UserID)
	if err != nil {
		h.logClassroomError("get student class failed", err)
		writeClassError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取当前班级失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	token, ok := httpauth.BearerToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeClassError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	principal, ok := h.auth.DecodeAccessToken(token)
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeClassError(w, http.StatusUnauthorized, "UNAUTHORIZED", "未认证，请先登录")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) requireTeacher(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return authapp.Principal{}, false
	}
	if !authapp.IsTeacherOrAdmin(principal) {
		writeClassError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要教师权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) requireStudent(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return authapp.Principal{}, false
	}
	if !authapp.IsStudent(principal) {
		writeClassError(w, http.StatusForbidden, "FORBIDDEN", "权限不足，需要学生权限")
		return authapp.Principal{}, false
	}
	return principal, true
}

func (h *Handler) logClassroomError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func validateClassName(w http.ResponseWriter, value string) bool {
	length := len(strings.TrimSpace(value))
	if length >= 2 && length <= 200 {
		return true
	}
	writeClassError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "name 长度必须在 2 到 200 之间")
	return false
}

func validateClassCode(w http.ResponseWriter, value string) bool {
	length := len(strings.TrimSpace(value))
	if length >= 4 && length <= 12 {
		return true
	}
	writeClassError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "code 长度必须在 4 到 12 之间")
	return false
}

func validateOptionalLength(w http.ResponseWriter, value *string, max int, name string) bool {
	if value == nil || len(*value) <= max {
		return true
	}
	writeClassError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 长度超出限制")
	return false
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := httpjson.DecodeStrict(w, r, maxJSONBodyBytes, target); err != nil {
		writeClassError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体不是有效 JSON")
		return false
	}
	return true
}

func writeClassError(w http.ResponseWriter, status int, code, message string) {
	httpjson.Write(w, status, errorResponse{Detail: message, Code: code, Message: message})
}
