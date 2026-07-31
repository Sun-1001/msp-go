package progresshttp

import (
	"errors"
	"net/http"

	progressapp "mathstudy/backend/internal/application/progress"
	"mathstudy/backend/internal/platform/httpjson"
)

func (h *Handler) portraitInsights(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r, "权限不足，仅学生可以访问学生画像")
	if !ok {
		return
	}
	rangeType := r.URL.Query().Get("range")
	if rangeType == "" {
		rangeType = "week"
	}
	response, err := h.service.GetPortraitInsights(r.Context(), principal.UserID, rangeType)
	if err != nil {
		if errors.Is(err, progressapp.ErrInvalidPortraitRange) {
			writeProgressError(w, http.StatusUnprocessableEntity, "INVALID_RANGE", "画像时间范围无效")
			return
		}
		h.logProgressError("get portrait insights failed", err)
		writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "获取学生画像洞察失败")
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) startPortraitAction(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.requireStudent(w, r, "权限不足，仅学生可以开始画像行动")
	if !ok {
		return
	}
	response, err := h.service.StartPortraitAction(r.Context(), principal.UserID, r.PathValue("concept_id"))
	if err != nil {
		switch {
		case errors.Is(err, progressapp.ErrInvalidPortraitAction):
			writeProgressError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "concept_id 不能为空")
		case errors.Is(err, progressapp.ErrPortraitActionConceptNotFound):
			writeProgressError(w, http.StatusNotFound, "NOT_FOUND", "知识节点不存在")
		default:
			h.logProgressError("start portrait action failed", err)
			writeProgressError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "开始画像行动失败")
		}
		return
	}
	httpjson.Write(w, http.StatusOK, response)
}
