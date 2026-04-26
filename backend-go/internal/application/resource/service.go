package resource

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrNotFound is returned when a resource does not exist or is not accessible.
var ErrNotFound = errors.New("resource not found")

// Repository is the persistence surface required by resource center use cases.
type Repository interface {
	ListResources(context.Context, string, ListFilter) ([]Resource, int, error)
	GetResourceByID(context.Context, string, string) (Resource, bool, error)
	CreateResource(context.Context, string, ResourceInput, time.Time) (Resource, error)
	UpdateResource(context.Context, string, string, ResourceUpdate, time.Time) (Resource, bool, error)
	DeleteResource(context.Context, string, string, time.Time) (bool, error)
	ToggleFavorite(context.Context, string, string) (bool, bool, error)
	GetStats(context.Context, string) (Stats, error)
}

// ListFilter stores /resources filters and pagination.
type ListFilter struct {
	Type          string
	Chapter       string
	Topic         string
	Search        string
	FavoritesOnly bool
	Page          int
	PageSize      int
}

// ResourceInput stores fields required to create a resource.
type ResourceInput struct {
	Title       string
	Type        string
	Body        string
	Chapter     *string
	Topic       *string
	Tags        []string
	Difficulty  float64
	StorageType string
	URL         *string
	Duration    *string
	Pages       *int
	Source      *string
}

// ResourceUpdate stores optional fields accepted by update resource.
type ResourceUpdate struct {
	Title       *string
	Type        *string
	Body        *string
	Chapter     *string
	Topic       *string
	Tags        []string
	TagsSet     bool
	Difficulty  *float64
	StorageType *string
	URL         *string
	Duration    *string
	Pages       *int
	Source      *string
}

// Resource is the Python-compatible API response shape.
type Resource struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Type        string    `json:"type"`
	Body        string    `json:"body"`
	Chapter     *string   `json:"chapter"`
	Topic       *string   `json:"topic"`
	Tags        []string  `json:"tags"`
	Difficulty  float64   `json:"difficulty"`
	Source      *string   `json:"source"`
	URL         *string   `json:"url"`
	StorageType *string   `json:"storage_type"`
	Duration    *string   `json:"duration"`
	Pages       *int      `json:"pages"`
	Views       int       `json:"views"`
	Likes       int       `json:"likes"`
	IsFavorite  bool      `json:"is_favorite"`
	OwnerID     string    `json:"owner_id"`
	OwnerName   *string   `json:"owner_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListResponse is the Python-compatible resource list response.
type ListResponse struct {
	Items    []Resource `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	HasMore  bool       `json:"has_more"`
}

// Stats stores resource counters.
type Stats struct {
	Total     int `json:"total"`
	Videos    int `json:"videos"`
	Documents int `json:"documents"`
	Favorites int `json:"favorites"`
}

// FavoriteToggleResponse is returned after toggling one favorite.
type FavoriteToggleResponse struct {
	ResourceID string `json:"resource_id"`
	IsFavorite bool   `json:"is_favorite"`
	Message    string `json:"message"`
}

// Service implements resource center use cases.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService creates a resource service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("resource repository is nil")
	}
	return &Service{repo: repo, now: time.Now}, nil
}

// GetResources returns a filtered resource page.
func (s *Service) GetResources(ctx context.Context, userID string, filter ListFilter) (ListResponse, error) {
	filter = normalizeListFilter(filter)
	items, total, err := s.repo.ListResources(ctx, userID, filter)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{
		Items:    items,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		HasMore:  filter.Page*filter.PageSize < total,
	}, nil
}

// GetFavorites returns only the current user's favorite resources.
func (s *Service) GetFavorites(ctx context.Context, userID string, page int, pageSize int) (ListResponse, error) {
	return s.GetResources(ctx, userID, ListFilter{FavoritesOnly: true, Page: page, PageSize: pageSize})
}

// GetResource returns one published resource and records a view in the repository.
func (s *Service) GetResource(ctx context.Context, userID string, resourceID string) (Resource, error) {
	resource, ok, err := s.repo.GetResourceByID(ctx, resourceID, userID)
	if err != nil {
		return Resource{}, err
	}
	if !ok {
		return Resource{}, ErrNotFound
	}
	return resource, nil
}

// CreateResource creates a teacher-owned published resource.
func (s *Service) CreateResource(ctx context.Context, ownerID string, input ResourceInput) (Resource, error) {
	input = normalizeResourceInput(input)
	return s.repo.CreateResource(ctx, ownerID, input, s.now())
}

// UpdateResource updates a teacher-owned resource.
func (s *Service) UpdateResource(ctx context.Context, resourceID string, ownerID string, input ResourceUpdate) (Resource, error) {
	resource, ok, err := s.repo.UpdateResource(ctx, resourceID, ownerID, input, s.now())
	if err != nil {
		return Resource{}, err
	}
	if !ok {
		return Resource{}, ErrNotFound
	}
	return resource, nil
}

// DeleteResource soft-deletes a teacher-owned resource.
func (s *Service) DeleteResource(ctx context.Context, resourceID string, ownerID string) error {
	ok, err := s.repo.DeleteResource(ctx, resourceID, ownerID, s.now())
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// ToggleFavorite toggles the favorite relation for the current user.
func (s *Service) ToggleFavorite(ctx context.Context, userID string, resourceID string) (FavoriteToggleResponse, error) {
	isFavorite, ok, err := s.repo.ToggleFavorite(ctx, userID, resourceID)
	if err != nil {
		return FavoriteToggleResponse{}, err
	}
	if !ok {
		return FavoriteToggleResponse{}, ErrNotFound
	}
	message := "已取消收藏"
	if isFavorite {
		message = "已收藏"
	}
	return FavoriteToggleResponse{ResourceID: resourceID, IsFavorite: isFavorite, Message: message}, nil
}

// GetStats returns resource center counters for the current user.
func (s *Service) GetStats(ctx context.Context, userID string) (Stats, error) {
	return s.repo.GetStats(ctx, userID)
}

func normalizeListFilter(filter ListFilter) ListFilter {
	filter.Type = strings.TrimSpace(filter.Type)
	filter.Chapter = strings.TrimSpace(filter.Chapter)
	filter.Topic = strings.TrimSpace(filter.Topic)
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return filter
}

func normalizeResourceInput(input ResourceInput) ResourceInput {
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	input.StorageType = strings.ToLower(strings.TrimSpace(input.StorageType))
	if input.StorageType == "" {
		input.StorageType = "external"
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	return input
}
