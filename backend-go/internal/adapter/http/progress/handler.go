package progresshttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authapp "mathstudy/backend-go/internal/application/auth"
	progressapp "mathstudy/backend-go/internal/application/progress"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the progress application surface used by HTTP handlers.
type Service interface {
	GetOverview(context.Context, string) (progressapp.Overview, error)
	GetMasteryVector(context.Context, string) (progressapp.MasteryResponse, error)
	GetLearningPath(context.Context, string, string) (progressapp.PathResponse, error)
	GetKnowledgeGraphView(context.Context, string, progressapp.KnowledgeNodeFilter) (progressapp.GraphResponse, error)
	GetStatistics(context.Context, string, string) (progressapp.StatisticsResponse, error)
	GetClassRanking(context.Context, string) (progressapp.ClassRankingResponse, error)
	GetChapters(context.Context) ([]string, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /progress endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates a progress HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("progress service is nil")
	}
	if auth == nil {
		return nil, errors.New("progress authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches progress routes under prefix, for example /api/v1/progress.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/overview", h.overview)
	mux.HandleFunc("GET "+prefix+"/mastery", h.mastery)
	mux.HandleFunc("GET "+prefix+"/path", h.path)
	mux.HandleFunc("GET "+prefix+"/knowledge-graph", h.knowledgeGraph)
	mux.HandleFunc("GET "+prefix+"/statistics", h.statistics)
	mux.HandleFunc("GET "+prefix+"/class-ranking", h.classRanking)
	mux.HandleFunc("GET "+prefix+"/chapters", h.chapters)
}

type chaptersResponse struct {
	Chapters []string `json:"chapters"`
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetOverview(r.Context(), principal.UserID)
	if err != nil {
		h.logProgressError("get progress overview failed", err)
		writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取学习进度失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) mastery(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetMasteryVector(r.Context(), principal.UserID)
	if err != nil {
		h.logProgressError("get mastery vector failed", err)
		writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取知识点掌握度失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) path(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetLearningPath(r.Context(), principal.UserID, r.URL.Query().Get("target"))
	if err != nil {
		h.logProgressError("get learning path failed", err)
		writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取学习路径失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) knowledgeGraph(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	response, err := h.service.GetKnowledgeGraphView(r.Context(), principal.UserID, progressapp.KnowledgeNodeFilter{
		Chapter:  query.Get("chapter"),
		NodeType: query.Get("type"),
		Search:   query.Get("search"),
	})
	if err != nil {
		h.logProgressError("get knowledge graph failed", err)
		writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取知识图谱失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) statistics(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	rangeType := r.URL.Query().Get("range")
	if rangeType == "" {
		rangeType = "week"
	}
	response, err := h.service.GetStatistics(r.Context(), principal.UserID, rangeType)
	if err != nil {
		h.logProgressError("get learning statistics failed", err)
		writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取学习统计失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) classRanking(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requirePrincipal(w, r)
	if !ok {
		return
	}
	response, err := h.service.GetClassRanking(r.Context(), principal.UserID)
	if err != nil {
		h.logProgressError("get class ranking failed", err)
		writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取班级排名失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) chapters(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePrincipal(w, r); !ok {
		return
	}
	chapters, err := h.service.GetChapters(r.Context())
	if err != nil {
		h.logProgressError("get chapters failed", err)
		writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取章节列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, chaptersResponse{Chapters: chapters})
}

func (h *Handler) requirePrincipal(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(w, r, h.auth.DecodeAccessToken, nil, "", writeProgressError)
}

func (h *Handler) logProgressError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func writeProgressError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
