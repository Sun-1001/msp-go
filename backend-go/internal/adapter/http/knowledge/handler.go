package knowledgehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	authapp "mathstudy/backend-go/internal/application/auth"
	knowledgeapp "mathstudy/backend-go/internal/application/knowledge"
	"mathstudy/backend-go/internal/platform/httpauth"
	"mathstudy/backend-go/internal/platform/httpjson"
	"mathstudy/backend-go/internal/platform/httpquery"
	"mathstudy/backend-go/internal/platform/httpvalidate"
	"mathstudy/backend-go/internal/platform/redact"
)

// Service is the admin knowledge application surface used by HTTP handlers.
type Service interface {
	GetStats(context.Context) (knowledgeapp.Stats, error)
	GetChapters(context.Context) ([]string, error)
	ListNodes(context.Context, knowledgeapp.NodeFilter) (knowledgeapp.NodeListResponse, error)
	GetAllNodesSimple(context.Context) ([]knowledgeapp.SimpleNode, error)
	GetNode(context.Context, string) (knowledgeapp.KnowledgeNode, error)
	CreateNode(context.Context, knowledgeapp.NodeInput) (knowledgeapp.NodeResponse, error)
	UpdateNode(context.Context, string, knowledgeapp.NodeUpdate) (knowledgeapp.NodeResponse, error)
	DeleteNode(context.Context, string) (knowledgeapp.DeleteResponse, error)
	ListRelations(context.Context, string) (knowledgeapp.RelationListResponse, error)
	CreateRelation(context.Context, knowledgeapp.RelationInput) (knowledgeapp.RelationResponse, error)
	UpdateRelation(context.Context, string, knowledgeapp.RelationUpdate) (knowledgeapp.RelationResponse, error)
	DeleteRelation(context.Context, string) (knowledgeapp.DeleteResponse, error)
}

// Authenticator decodes Go/Python-compatible access tokens.
type Authenticator interface {
	DecodeAccessToken(string) (authapp.Principal, bool)
}

// Handler serves /admin/knowledge endpoints.
type Handler struct {
	service Service
	auth    Authenticator
	logger  *slog.Logger
}

// NewHandler creates an admin knowledge HTTP handler.
func NewHandler(logger *slog.Logger, service Service, auth Authenticator) (*Handler, error) {
	if service == nil {
		return nil, errors.New("knowledge service is nil")
	}
	if auth == nil {
		return nil, errors.New("knowledge authenticator is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, auth: auth, logger: logger}, nil
}

// Register attaches knowledge routes under prefix, for example /api/v1/admin/knowledge.
func (h *Handler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix+"/stats", h.stats)
	mux.HandleFunc("GET "+prefix+"/chapters", h.chapters)
	mux.HandleFunc("GET "+prefix+"/nodes", h.listNodes)
	mux.HandleFunc("GET "+prefix+"/nodes/all", h.allNodes)
	mux.HandleFunc("POST "+prefix+"/nodes", h.createNode)
	mux.HandleFunc("GET "+prefix+"/nodes/{node_id}", h.getNode)
	mux.HandleFunc("PUT "+prefix+"/nodes/{node_id}", h.updateNode)
	mux.HandleFunc("DELETE "+prefix+"/nodes/{node_id}", h.deleteNode)
	mux.HandleFunc("GET "+prefix+"/relations", h.listRelations)
	mux.HandleFunc("POST "+prefix+"/relations", h.createRelation)
	mux.HandleFunc("PUT "+prefix+"/relations/{relation_id}", h.updateRelation)
	mux.HandleFunc("DELETE "+prefix+"/relations/{relation_id}", h.deleteRelation)
}

type nodeCreateRequest struct {
	Name         string   `json:"name"`
	NameEn       *string  `json:"name_en"`
	NodeType     string   `json:"node_type"`
	Description  string   `json:"description"`
	Chapter      *string  `json:"chapter"`
	Section      *string  `json:"section"`
	Difficulty   *float64 `json:"difficulty"`
	LatexFormula *string  `json:"latex_formula"`
	Tags         []string `json:"tags"`
}

type nodeUpdateRequest struct {
	Name         *string   `json:"name"`
	NameEn       *string   `json:"name_en"`
	NodeType     *string   `json:"node_type"`
	Description  *string   `json:"description"`
	Chapter      *string   `json:"chapter"`
	Section      *string   `json:"section"`
	Difficulty   *float64  `json:"difficulty"`
	LatexFormula *string   `json:"latex_formula"`
	Tags         *[]string `json:"tags"`
}

type relationCreateRequest struct {
	SourceID     string   `json:"source_id"`
	TargetID     string   `json:"target_id"`
	RelationType string   `json:"relation_type"`
	Weight       *float64 `json:"weight"`
	Description  *string  `json:"description"`
}

type relationUpdateRequest struct {
	RelationType *string  `json:"relation_type"`
	Weight       *float64 `json:"weight"`
	Description  *string  `json:"description"`
}

type chaptersResponse struct {
	Chapters []string `json:"chapters"`
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.GetStats(r.Context())
	if err != nil {
		h.logKnowledgeError("get knowledge stats failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取知识点统计失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) chapters(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	chapters, err := h.service.GetChapters(r.Context())
	if err != nil {
		h.logKnowledgeError("get knowledge chapters failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取章节列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, chaptersResponse{Chapters: chapters})
}

func (h *Handler) listNodes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	filter, ok := parseNodeFilter(w, r)
	if !ok {
		return
	}
	response, err := h.service.ListNodes(r.Context(), filter)
	if err != nil {
		if errors.Is(err, knowledgeapp.ErrBadRequest) {
			writeKnowledgeAppError(w, http.StatusBadRequest, "BAD_REQUEST", err)
			return
		}
		h.logKnowledgeError("list knowledge nodes failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取知识节点列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) allNodes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.GetAllNodesSimple(r.Context())
	if err != nil {
		h.logKnowledgeError("get all simple nodes failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取节点简要信息失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) getNode(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.GetNode(r.Context(), r.PathValue("node_id"))
	if err != nil {
		if errors.Is(err, knowledgeapp.ErrNotFound) {
			writeKnowledgeAppError(w, http.StatusNotFound, "NOT_FOUND", err)
			return
		}
		h.logKnowledgeError("get knowledge node failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取知识节点失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) createNode(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request nodeCreateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	input, ok := request.toInput(w)
	if !ok {
		return
	}
	response, err := h.service.CreateNode(r.Context(), input)
	if err != nil {
		if errors.Is(err, knowledgeapp.ErrBadRequest) {
			writeKnowledgeAppError(w, http.StatusBadRequest, "BAD_REQUEST", err)
			return
		}
		h.logKnowledgeError("create knowledge node failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "创建知识节点失败")
		return
	}
	httpjson.Write(w, http.StatusCreated, response)
}

func (h *Handler) updateNode(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request nodeUpdateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	update, ok := request.toUpdate(w)
	if !ok {
		return
	}
	response, err := h.service.UpdateNode(r.Context(), r.PathValue("node_id"), update)
	if err != nil {
		if errors.Is(err, knowledgeapp.ErrBadRequest) || errors.Is(err, knowledgeapp.ErrNotFound) {
			writeKnowledgeAppError(w, http.StatusBadRequest, "BAD_REQUEST", err)
			return
		}
		h.logKnowledgeError("update knowledge node failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "更新知识节点失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) deleteNode(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.DeleteNode(r.Context(), r.PathValue("node_id"))
	if err != nil {
		if errors.Is(err, knowledgeapp.ErrBadRequest) {
			writeKnowledgeAppError(w, http.StatusBadRequest, "BAD_REQUEST", err)
			return
		}
		if errors.Is(err, knowledgeapp.ErrNotFound) {
			writeKnowledgeAppError(w, http.StatusNotFound, "NOT_FOUND", err)
			return
		}
		h.logKnowledgeError("delete knowledge node failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "删除知识节点失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) listRelations(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.ListRelations(r.Context(), r.URL.Query().Get("node_id"))
	if err != nil {
		h.logKnowledgeError("list knowledge relations failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取知识关系列表失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) createRelation(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request relationCreateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	input, ok := request.toInput(w)
	if !ok {
		return
	}
	response, err := h.service.CreateRelation(r.Context(), input)
	if err != nil {
		if errors.Is(err, knowledgeapp.ErrBadRequest) {
			writeKnowledgeAppError(w, http.StatusBadRequest, "BAD_REQUEST", err)
			return
		}
		h.logKnowledgeError("create knowledge relation failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "创建知识关系失败")
		return
	}
	httpjson.Write(w, http.StatusCreated, response)
}

func (h *Handler) updateRelation(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var request relationUpdateRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	update, ok := request.toUpdate(w)
	if !ok {
		return
	}
	response, err := h.service.UpdateRelation(r.Context(), r.PathValue("relation_id"), update)
	if err != nil {
		if errors.Is(err, knowledgeapp.ErrBadRequest) || errors.Is(err, knowledgeapp.ErrNotFound) {
			writeKnowledgeAppError(w, http.StatusBadRequest, "BAD_REQUEST", err)
			return
		}
		h.logKnowledgeError("update knowledge relation failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "更新知识关系失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) deleteRelation(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	response, err := h.service.DeleteRelation(r.Context(), r.PathValue("relation_id"))
	if err != nil {
		if errors.Is(err, knowledgeapp.ErrNotFound) {
			writeKnowledgeAppError(w, http.StatusNotFound, "NOT_FOUND", err)
			return
		}
		h.logKnowledgeError("delete knowledge relation failed", err)
		writeKnowledgeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "删除知识关系失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (authapp.Principal, bool) {
	return httpauth.RequireBearerAccess(
		w, r, h.auth.DecodeAccessToken, authapp.IsAdmin,
		"权限不足，需要管理员权限", writeKnowledgeError,
	)
}

func (h *Handler) logKnowledgeError(message string, err error) {
	h.logger.Error(message, "error", redact.String(err.Error()))
}

func writeKnowledgeAppError(w http.ResponseWriter, status int, code string, err error) {
	writeKnowledgeError(w, status, code, redact.String(err.Error()))
}

func parseNodeFilter(w http.ResponseWriter, r *http.Request) (knowledgeapp.NodeFilter, bool) {
	query := r.URL.Query()
	pagination, err := httpquery.Pagination(query, 20, 100)
	if err != nil {
		writeKnowledgePaginationError(w, err)
		return knowledgeapp.NodeFilter{}, false
	}
	nodeType := query.Get("type")
	if nodeType != "" && !validNodeType(nodeType) {
		writeKnowledgeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "type 节点类型无效")
		return knowledgeapp.NodeFilter{}, false
	}
	return knowledgeapp.NodeFilter{
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
		Chapter:  query.Get("chapter"),
		NodeType: nodeType,
		Search:   query.Get("search"),
	}, true
}

func writeKnowledgePaginationError(w http.ResponseWriter, err error) {
	writeKnowledgeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", httpquery.PaginationErrorMessage(err, 100))
}

func (r nodeCreateRequest) toInput(w http.ResponseWriter) (knowledgeapp.NodeInput, bool) {
	if !httpvalidate.RequiredTrimmedString(w, r.Name, 1, 200, "name", writeKnowledgeError) ||
		!validateNodeType(w, r.NodeType, "node_type") ||
		!httpvalidate.StringLength(w, r.Description, 2000, "description", writeKnowledgeError) ||
		!httpvalidate.OptionalString(w, r.NameEn, 200, "name_en", writeKnowledgeError) ||
		!httpvalidate.OptionalString(w, r.Chapter, 100, "chapter", writeKnowledgeError) ||
		!httpvalidate.OptionalString(w, r.Section, 100, "section", writeKnowledgeError) ||
		!validateDifficulty(w, r.Difficulty) {
		return knowledgeapp.NodeInput{}, false
	}
	difficulty := 0.5
	if r.Difficulty != nil {
		difficulty = *r.Difficulty
	}
	if r.Tags == nil {
		r.Tags = []string{}
	}
	return knowledgeapp.NodeInput{
		Name:         r.Name,
		NameEn:       r.NameEn,
		NodeType:     r.NodeType,
		Description:  r.Description,
		Chapter:      r.Chapter,
		Section:      r.Section,
		Difficulty:   difficulty,
		LatexFormula: r.LatexFormula,
		Tags:         r.Tags,
	}, true
}

func (r nodeUpdateRequest) toUpdate(w http.ResponseWriter) (knowledgeapp.NodeUpdate, bool) {
	if !httpvalidate.OptionalRequiredTrimmedString(w, r.Name, 1, 200, "name", writeKnowledgeError) ||
		!validateOptionalNodeType(w, r.NodeType, "node_type") ||
		!httpvalidate.OptionalString(w, r.Description, 2000, "description", writeKnowledgeError) ||
		!httpvalidate.OptionalString(w, r.NameEn, 200, "name_en", writeKnowledgeError) ||
		!httpvalidate.OptionalString(w, r.Chapter, 100, "chapter", writeKnowledgeError) ||
		!httpvalidate.OptionalString(w, r.Section, 100, "section", writeKnowledgeError) ||
		!validateDifficulty(w, r.Difficulty) {
		return knowledgeapp.NodeUpdate{}, false
	}
	return knowledgeapp.NodeUpdate{
		Name:         r.Name,
		NameEn:       r.NameEn,
		NodeType:     r.NodeType,
		Description:  r.Description,
		Chapter:      r.Chapter,
		Section:      r.Section,
		Difficulty:   r.Difficulty,
		LatexFormula: r.LatexFormula,
		Tags:         r.Tags,
	}, true
}

func (r relationCreateRequest) toInput(w http.ResponseWriter) (knowledgeapp.RelationInput, bool) {
	if !httpvalidate.RequiredTrimmedString(w, r.SourceID, 1, 0, "source_id", writeKnowledgeError) ||
		!httpvalidate.RequiredTrimmedString(w, r.TargetID, 1, 0, "target_id", writeKnowledgeError) ||
		!validateRelationType(w, r.RelationType, "relation_type") ||
		!validateWeight(w, r.Weight) ||
		!httpvalidate.OptionalString(w, r.Description, 500, "description", writeKnowledgeError) {
		return knowledgeapp.RelationInput{}, false
	}
	weight := 1.0
	if r.Weight != nil {
		weight = *r.Weight
	}
	return knowledgeapp.RelationInput{
		SourceID:     r.SourceID,
		TargetID:     r.TargetID,
		RelationType: r.RelationType,
		Weight:       weight,
		Description:  r.Description,
	}, true
}

func (r relationUpdateRequest) toUpdate(w http.ResponseWriter) (knowledgeapp.RelationUpdate, bool) {
	if !validateOptionalRelationType(w, r.RelationType, "relation_type") ||
		!validateWeight(w, r.Weight) ||
		!httpvalidate.OptionalString(w, r.Description, 500, "description", writeKnowledgeError) {
		return knowledgeapp.RelationUpdate{}, false
	}
	return knowledgeapp.RelationUpdate{
		RelationType: r.RelationType,
		Weight:       r.Weight,
		Description:  r.Description,
	}, true
}

func validateDifficulty(w http.ResponseWriter, value *float64) bool {
	if value == nil || (*value >= 0 && *value <= 1) {
		return true
	}
	writeKnowledgeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "difficulty 必须在 0 到 1 之间")
	return false
}

func validateWeight(w http.ResponseWriter, value *float64) bool {
	if value == nil || (*value >= 0 && *value <= 1) {
		return true
	}
	writeKnowledgeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "weight 必须在 0 到 1 之间")
	return false
}

func validateNodeType(w http.ResponseWriter, value string, name string) bool {
	if validNodeType(value) {
		return true
	}
	writeKnowledgeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 节点类型无效")
	return false
}

func validateOptionalNodeType(w http.ResponseWriter, value *string, name string) bool {
	if value == nil {
		return true
	}
	return validateNodeType(w, *value, name)
}

func validNodeType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "concept", "theorem", "method", "problem", "misconception", "resource":
		return true
	default:
		return false
	}
}

func validateRelationType(w http.ResponseWriter, value string, name string) bool {
	if validRelationType(value) {
		return true
	}
	writeKnowledgeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", name+" 关系类型无效")
	return false
}

func validateOptionalRelationType(w http.ResponseWriter, value *string, name string) bool {
	if value == nil {
		return true
	}
	return validateRelationType(w, *value, name)
}

func validRelationType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "has_prerequisite", "is_a_special_case_of", "used_in", "prone_to_error", "related_to":
		return true
	default:
		return false
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	return httpjson.DecodeStrictOrDetailError(w, r, 2<<20, target)
}

func writeKnowledgeError(w http.ResponseWriter, status int, code, message string) {
	httpjson.WriteDetailError(w, status, code, message)
}
