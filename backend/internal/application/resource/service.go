package resource

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"mathstudy/backend/internal/platform/outbound"
	"mathstudy/backend/internal/platform/uploadpath"
)

// ErrNotFound is returned when a resource does not exist or is not accessible.
var ErrNotFound = errors.New("resource not found")

// ErrBadRequest is returned when resource input fails application-level validation.
var ErrBadRequest = errors.New("bad resource request")

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
	input, err := normalizeResourceInput(input)
	if err != nil {
		return Resource{}, err
	}
	return s.repo.CreateResource(ctx, ownerID, input, s.now())
}

// UpdateResource updates a teacher-owned resource.
func (s *Service) UpdateResource(ctx context.Context, resourceID string, ownerID string, input ResourceUpdate) (Resource, error) {
	var err error
	input, err = normalizeResourceUpdate(input)
	if err != nil {
		return Resource{}, err
	}
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

func normalizeResourceInput(input ResourceInput) (ResourceInput, error) {
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	input.StorageType = strings.ToLower(strings.TrimSpace(input.StorageType))
	if input.StorageType == "" {
		input.StorageType = "external"
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	normalizedURL, err := normalizeOptionalResourceURL(input.URL, input.StorageType, false)
	if err != nil {
		return ResourceInput{}, err
	}
	input.URL = normalizedURL
	return input, nil
}

func normalizeResourceUpdate(input ResourceUpdate) (ResourceUpdate, error) {
	storageType := "external"
	if input.Type != nil {
		value := strings.ToLower(strings.TrimSpace(*input.Type))
		input.Type = &value
	}
	if input.StorageType != nil {
		value := strings.ToLower(strings.TrimSpace(*input.StorageType))
		input.StorageType = &value
		if value != "" {
			storageType = value
		}
	}
	normalizedURL, err := normalizeOptionalResourceURL(input.URL, storageType, true)
	if err != nil {
		return ResourceUpdate{}, err
	}
	input.URL = normalizedURL
	return input, nil
}

func normalizeOptionalResourceURL(value *string, storageType string, preserveEmpty bool) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized, err := normalizeResourceURL(*value, storageType)
	if err != nil {
		return nil, err
	}
	if normalized == "" && !preserveEmpty {
		return nil, nil
	}
	return &normalized, nil
}

func normalizeResourceURL(value string, storageType string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if len([]rune(trimmed)) > 1000 {
		return "", resourceValidationError("资源链接长度超出限制")
	}
	if strings.Contains(trimmed, "\\") || strings.IndexFunc(trimmed, unicode.IsSpace) >= 0 || strings.IndexFunc(trimmed, unicode.IsControl) >= 0 {
		return "", resourceValidationError("资源链接格式无效")
	}
	if strings.HasPrefix(trimmed, "/") {
		if uploadpath.IsResourcePath(trimmed) {
			return trimmed, nil
		}
		return "", resourceValidationError("本地资源链接必须是已上传的文档或视频")
	}
	if strings.HasPrefix(trimmed, "//") {
		return "", resourceValidationError("资源链接必须包含 http 或 https 协议")
	}
	if !hasURLScheme(trimmed) && hasNonPortColonBeforePath(trimmed) {
		return "", resourceValidationError("资源链接仅支持 http 或 https")
	}

	parsed, err := parseResourceURL(trimmed)
	if err != nil {
		return "", resourceValidationError("资源链接格式无效")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", resourceValidationError("资源链接仅支持 http 或 https")
	}
	if parsed.User != nil {
		return "", resourceValidationError("资源链接不允许包含用户名或密码")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return "", resourceValidationError("资源链接必须包含主机名")
	}
	if err := validateResourceURLPort(parsed.Port()); err != nil {
		return "", err
	}
	if strings.ToLower(strings.TrimSpace(storageType)) == "external" && outbound.IsBlockedPublicProviderHost(parsed.Hostname()) {
		return "", resourceValidationError("外部资源链接不允许指向本机、内网或保留地址")
	}
	return parsed.String(), nil
}

func parseResourceURL(value string) (*url.URL, error) {
	if !hasURLScheme(value) {
		return url.Parse("https://" + value)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func hasURLScheme(value string) bool {
	scheme, _, found := strings.Cut(value, "://")
	if !found || scheme == "" {
		return false
	}
	for i := 0; i < len(scheme); i++ {
		c := scheme[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (i > 0 && ((c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.')) {
			continue
		}
		return false
	}
	return true
}

func hasNonPortColonBeforePath(value string) bool {
	index := strings.IndexAny(value, ":/?#")
	if index < 0 || value[index] != ':' {
		return false
	}
	portStart := index + 1
	portEnd := portStart
	for portEnd < len(value) && value[portEnd] >= '0' && value[portEnd] <= '9' {
		portEnd++
	}
	if portEnd == portStart {
		return true
	}
	if portEnd == len(value) {
		return false
	}
	switch value[portEnd] {
	case '/', '?', '#':
		return false
	default:
		return true
	}
}

func validateResourceURLPort(port string) error {
	if port == "" {
		return nil
	}
	parsed, err := strconv.Atoi(port)
	if err != nil || parsed < 1 || parsed > 65535 {
		return resourceValidationError("资源链接端口无效")
	}
	return nil
}

func resourceValidationError(message string) error {
	return fmt.Errorf("%w: %s", ErrBadRequest, message)
}
