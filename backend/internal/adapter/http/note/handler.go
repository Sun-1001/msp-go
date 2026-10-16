package notehttp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	authapp "mathstudy/backend/internal/application/auth"
	noteapp "mathstudy/backend/internal/application/note"
	uploadapp "mathstudy/backend/internal/application/upload"
	"mathstudy/backend/internal/platform/httpauth"
	"mathstudy/backend/internal/platform/httpjson"
	"mathstudy/backend/internal/platform/ratelimit"
	"mathstudy/backend/internal/platform/redact"
)

type Service interface {
	List(context.Context, string, string) ([]noteapp.Summary, error)
	Get(context.Context, string, string) (noteapp.Note, error)
	Save(context.Context, string, string, noteapp.Input) (noteapp.Note, error)
	Delete(context.Context, string, string, int64) error
	Network(context.Context, string, string) (noteapp.Network, error)
	ChangeLink(context.Context, string, string, string, int64, bool) (noteapp.Note, error)
	UploadMaterial(context.Context, string, string, io.Reader) (noteapp.Material, error)
	ListMaterials(context.Context, string) ([]noteapp.Material, error)
	OpenMaterial(context.Context, string, string) (noteapp.Material, io.ReadCloser, error)
}
type Authenticator interface {
	DecodeActiveAccessToken(context.Context, string) (authapp.Principal, bool, error)
}
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
	limiter *ratelimit.Limiter
}

func NewHandler(logger *slog.Logger, service Service, auth Authenticator, limiter *ratelimit.Limiter) (*Handler, error) {
	if service == nil || auth == nil || limiter == nil {
		return nil, errors.New("note handler dependency is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service, auth, logger, limiter}, nil
}
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix, h.handle)
	mux.HandleFunc("POST "+prefix, h.handle)
	mux.HandleFunc("GET "+prefix+"/network", h.network)
	mux.HandleFunc("GET "+prefix+"/materials", h.materials)
	mux.HandleFunc("POST "+prefix+"/materials", h.materials)
	mux.HandleFunc("GET "+prefix+"/materials/{material_id}/file", h.download)
	mux.HandleFunc("GET "+prefix+"/{note_id}", h.handle)
	mux.HandleFunc("PUT "+prefix+"/{note_id}", h.handle)
	mux.HandleFunc("DELETE "+prefix+"/{note_id}", h.handle)
	mux.HandleFunc("GET "+prefix+"/{note_id}/network", h.network)
	mux.HandleFunc("POST "+prefix+"/{note_id}/links/{target_id}", h.link)
	mux.HandleFunc("DELETE "+prefix+"/{note_id}/links/{target_id}", h.link)
}
func (h *Handler) principal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	return httpauth.RequireBearerAccessContext(w, r, h.auth.DecodeActiveAccessToken, authapp.IsStudent, "仅学生可以访问个人笔记", httpjson.WriteDetailError)
}
func (h *Handler) handle(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(w, r)
	if !ok {
		return
	}
	id := r.PathValue("note_id")
	var result any
	var err error
	status := http.StatusOK
	switch r.Method {
	case http.MethodGet:
		if id == "" {
			result, err = h.service.List(r.Context(), p.UserID, r.URL.Query().Get("search"))
		} else {
			result, err = h.service.Get(r.Context(), p.UserID, id)
		}
	case http.MethodPost, http.MethodPut:
		var input noteapp.Input
		if !httpjson.DecodeStrictOrDetailError(w, r, 1<<20, &input) {
			return
		}
		result, err = h.service.Save(r.Context(), p.UserID, id, input)
		if r.Method == http.MethodPost {
			status = http.StatusCreated
		}
	case http.MethodDelete:
		revision, parseErr := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		if parseErr != nil {
			h.fail(w, noteapp.ErrInvalid)
			return
		}
		err = h.service.Delete(r.Context(), p.UserID, id, revision)
		result = map[string]bool{"success": true}
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, status, result)
}
func (h *Handler) network(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(w, r)
	if !ok {
		return
	}
	result, err := h.service.Network(r.Context(), p.UserID, r.PathValue("note_id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
func (h *Handler) link(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(w, r)
	if !ok {
		return
	}
	var input struct {
		Revision int64 `json:"revision"`
	}
	if !httpjson.DecodeStrictOrDetailError(w, r, 1024, &input) {
		return
	}
	result, err := h.service.ChangeLink(r.Context(), p.UserID, r.PathValue("note_id"), r.PathValue("target_id"), input.Revision, r.Method == http.MethodPost)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
func (h *Handler) materials(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		result, err := h.service.ListMaterials(r.Context(), p.UserID)
		if err != nil {
			h.fail(w, err)
			return
		}
		httpjson.Write(w, http.StatusOK, result)
		return
	}
	if !h.limiter.Allow(r.Context(), p.UserID) {
		w.Header().Set("Retry-After", "60")
		httpjson.WriteDetailError(w, 429, "RATE_LIMITED", "上传过于频繁，请稍后重试")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, noteapp.MaxMaterialBytes+(1<<20))
	err := r.ParseMultipartForm(1 << 20)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.fail(w, uploadapp.ErrFileTooLarge)
		} else {
			h.fail(w, noteapp.ErrInvalid)
		}
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		h.fail(w, noteapp.ErrInvalid)
		return
	}
	defer file.Close()
	result, err := h.service.UploadMaterial(r.Context(), p.UserID, header.Filename, file)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, result)
}
func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(w, r)
	if !ok {
		return
	}
	material, reader, err := h.service.OpenMaterial(r.Context(), p.UserID, r.PathValue("material_id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", material.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": material.Filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, io.LimitReader(reader, noteapp.MaxMaterialBytes))
}
func (h *Handler) fail(w http.ResponseWriter, err error) {
	status, code, message := 500, "INTERNAL_ERROR", "笔记操作失败，请稍后重试"
	switch {
	case errors.Is(err, noteapp.ErrNotFound):
		status, code, message = 404, "NOTE_UNAVAILABLE", noteapp.ErrNotFound.Error()
	case errors.Is(err, noteapp.ErrConflict):
		status, code, message = 409, "NOTE_CONFLICT", noteapp.ErrConflict.Error()
	case errors.Is(err, noteapp.ErrLimit):
		status, code, message = 409, "NOTE_LIMIT", noteapp.ErrLimit.Error()
	case errors.Is(err, noteapp.ErrInvalid):
		status, code, message = 422, "VALIDATION_ERROR", noteapp.ErrInvalid.Error()
	case errors.Is(err, noteapp.ErrStorage):
		status, code, message = 503, "NOTE_STORAGE_UNAVAILABLE", noteapp.ErrStorage.Error()
	case errors.Is(err, uploadapp.ErrFileTooLarge):
		status, code, message = 413, "FILE_TOO_LARGE", "学习材料不能超过 10 MB"
	case errors.Is(err, uploadapp.ErrInvalidContentType):
		status, code, message = 415, "INVALID_CONTENT_TYPE", "请上传有效的 PDF、DOCX、TXT 或 Markdown 文件"
	default:
		h.logger.Error("note operation failed", "error", redact.String(err.Error()))
	}
	httpjson.WriteDetailError(w, status, code, message)
}
