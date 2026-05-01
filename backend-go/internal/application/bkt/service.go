package bkt

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrNotFound is returned when a concept BKT parameter row does not exist.
	ErrNotFound = errors.New("bkt parameter not found")
	// ErrBadRequest is returned when input cannot be applied.
	ErrBadRequest = errors.New("bad bkt request")
)

// Error wraps a domain error with a Python-compatible message.
type Error struct {
	Kind    error
	Message string
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Unwrap() error {
	return e.Kind
}

// Repository is the persistence surface required by BKT parameter management.
type Repository interface {
	ListParams(context.Context, int, int) ([]Param, int, error)
	UpdateParam(context.Context, string, Update, time.Time) (Param, bool, error)
	ResetParam(context.Context, string, Param, time.Time) (Param, bool, error)
	SeedDefaultParams(context.Context, Param, time.Time) (int, error)
}

// Param is the Python-compatible BKT parameter response shape.
type Param struct {
	ConceptID string  `json:"concept_id"`
	PL0       float64 `json:"p_l0"`
	PT        float64 `json:"p_t"`
	PG        float64 `json:"p_g"`
	PS        float64 `json:"p_s"`
}

// Update stores optional BKT parameter updates.
type Update struct {
	PL0 *float64
	PT  *float64
	PG  *float64
	PS  *float64
}

// ListResponse wraps paginated BKT parameter rows.
type ListResponse struct {
	Items  []Param `json:"items"`
	Total  int     `json:"total"`
	Offset int     `json:"offset"`
	Limit  int     `json:"limit"`
}

// SeedResponse reports default parameter seeding results.
type SeedResponse struct {
	SeededCount int    `json:"seeded_count"`
	Message     string `json:"message"`
}

// Service implements admin BKT parameter management.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService creates a BKT parameter service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("bkt repository is nil")
	}
	return &Service{repo: repo, now: time.Now}, nil
}

// DefaultParam returns the default concept BKT parameters.
func DefaultParam() Param {
	return Param{PL0: 0.25, PT: 0.12, PG: 0.20, PS: 0.10}
}

// ListParams returns a paginated BKT parameter list.
func (s *Service) ListParams(ctx context.Context, offset int, limit int) (ListResponse, error) {
	if offset < 0 {
		return ListResponse{}, badRequest("offset 必须大于等于 0")
	}
	if limit < 1 || limit > 200 {
		return ListResponse{}, badRequest("limit 必须在 1 到 200 之间")
	}
	items, total, err := s.repo.ListParams(ctx, offset, limit)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items, Total: total, Offset: offset, Limit: limit}, nil
}

// UpdateParam updates one concept's BKT parameters.
func (s *Service) UpdateParam(ctx context.Context, conceptID string, update Update) (Param, error) {
	conceptID = strings.TrimSpace(conceptID)
	if conceptID == "" {
		return Param{}, badRequest("concept_id 不能为空")
	}
	if err := validateUpdate(update); err != nil {
		return Param{}, err
	}
	param, ok, err := s.repo.UpdateParam(ctx, conceptID, update, s.now())
	if err != nil {
		return Param{}, err
	}
	if !ok {
		return Param{}, notFound(conceptID)
	}
	return param, nil
}

// ResetParam resets one concept's BKT parameters to defaults.
func (s *Service) ResetParam(ctx context.Context, conceptID string) (Param, error) {
	conceptID = strings.TrimSpace(conceptID)
	if conceptID == "" {
		return Param{}, badRequest("concept_id 不能为空")
	}
	param, ok, err := s.repo.ResetParam(ctx, conceptID, DefaultParam(), s.now())
	if err != nil {
		return Param{}, err
	}
	if !ok {
		return Param{}, notFound(conceptID)
	}
	return param, nil
}

// SeedDefaultParams creates default parameter rows for knowledge nodes missing them.
func (s *Service) SeedDefaultParams(ctx context.Context) (SeedResponse, error) {
	count, err := s.repo.SeedDefaultParams(ctx, DefaultParam(), s.now())
	if err != nil {
		return SeedResponse{}, err
	}
	message := "所有知识点已有 BKT 参数"
	if count > 0 {
		message = "已为 " + strconv.Itoa(count) + " 个知识点创建默认 BKT 参数"
	}
	return SeedResponse{SeededCount: count, Message: message}, nil
}

func validateUpdate(update Update) error {
	if err := validateProbability(update.PL0, 0, 1, "p_l0"); err != nil {
		return err
	}
	if err := validateProbability(update.PT, 0, 1, "p_t"); err != nil {
		return err
	}
	if err := validateProbability(update.PG, 0, 0.5, "p_g"); err != nil {
		return err
	}
	return validateProbability(update.PS, 0, 0.5, "p_s")
}

func validateProbability(value *float64, min float64, max float64, name string) error {
	if value == nil || (*value >= min && *value <= max) {
		return nil
	}
	return badRequest(name + " 必须在 " + formatProbability(min) + " 到 " + formatProbability(max) + " 之间")
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}

func notFound(conceptID string) error {
	return Error{Kind: ErrNotFound, Message: "知识点 " + conceptID + " 的 BKT 参数不存在"}
}

func formatProbability(value float64) string {
	switch value {
	case 0:
		return "0"
	case 0.5:
		return "0.5"
	case 1:
		return "1"
	default:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
}
