package knowledge

import (
	"context"
	"errors"
	"strings"
	"time"

	"mathstudy/backend/internal/platform/numutil"
)

var (
	// ErrNotFound is returned when a knowledge node or relation cannot be found.
	ErrNotFound = errors.New("knowledge not found")
	// ErrBadRequest is returned when an operation cannot be applied.
	ErrBadRequest = errors.New("knowledge bad request")
)

// Error wraps a domain error with the Python-compatible message.
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

// Repository is the persistence surface required by admin knowledge management.
type Repository interface {
	Stats(context.Context) (Stats, error)
	DistinctChapters(context.Context) ([]string, error)
	CountNodes(context.Context, NodeFilter) (int, error)
	ListNodes(context.Context, NodeFilter) ([]KnowledgeNode, error)
	ListAllSimpleNodes(context.Context) ([]SimpleNode, error)
	GetNode(context.Context, string) (KnowledgeNode, bool, error)
	CreateNode(context.Context, NodeInput, time.Time) (KnowledgeNode, error)
	UpdateNode(context.Context, string, NodeUpdate, time.Time) (KnowledgeNode, bool, error)
	DeleteNode(context.Context, string) (bool, error)
	NodeExists(context.Context, string) (bool, error)
	ListRelations(context.Context, string) ([]KnowledgeRelation, error)
	CreateRelation(context.Context, RelationInput, time.Time) (KnowledgeRelation, error)
	UpdateRelation(context.Context, string, RelationUpdate) (KnowledgeRelation, bool, error)
	DeleteRelation(context.Context, string) (bool, error)
}

// NodeFilter stores list node filters and pagination.
type NodeFilter struct {
	Page     int
	PageSize int
	Chapter  string
	NodeType string
	Search   string
}

// NodeInput stores fields required to create a knowledge node.
type NodeInput struct {
	Name         string
	NameEn       *string
	NodeType     string
	Description  string
	Chapter      *string
	Section      *string
	Difficulty   float64
	LatexFormula *string
	Tags         []string
}

// NodeUpdate stores optional knowledge node update fields.
type NodeUpdate struct {
	Name         *string
	NameEn       *string
	NodeType     *string
	Description  *string
	Chapter      *string
	Section      *string
	Difficulty   *float64
	LatexFormula *string
	Tags         *[]string
}

// RelationInput stores fields required to create a knowledge relation.
type RelationInput struct {
	SourceID     string
	TargetID     string
	RelationType string
	Weight       float64
	Description  *string
}

// RelationUpdate stores optional knowledge relation update fields.
type RelationUpdate struct {
	RelationType *string
	Weight       *float64
	Description  *string
}

// KnowledgeNode is the Python-compatible node response shape.
type KnowledgeNode struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	NameEn       *string   `json:"name_en"`
	NodeType     string    `json:"node_type"`
	Description  string    `json:"description"`
	Chapter      *string   `json:"chapter"`
	Section      *string   `json:"section"`
	Difficulty   float64   `json:"difficulty"`
	LatexFormula *string   `json:"latex_formula"`
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// KnowledgeRelation is the Python-compatible relation response shape.
type KnowledgeRelation struct {
	ID           string    `json:"id"`
	SourceID     string    `json:"source_id"`
	TargetID     string    `json:"target_id"`
	SourceName   *string   `json:"source_name"`
	TargetName   *string   `json:"target_name"`
	RelationType string    `json:"relation_type"`
	Weight       float64   `json:"weight"`
	Description  *string   `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

// SimpleNode stores the compact node shape used by relation selectors.
type SimpleNode struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Chapter  *string `json:"chapter"`
	NodeType *string `json:"node_type"`
}

// Stats stores knowledge graph counters.
type Stats struct {
	TotalNodes       int            `json:"total_nodes"`
	TotalRelations   int            `json:"total_relations"`
	ChaptersCount    int            `json:"chapters_count"`
	TypeDistribution map[string]int `json:"type_distribution"`
}

// NodeListResponse wraps paginated node rows.
type NodeListResponse struct {
	Items      []KnowledgeNode `json:"items"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
}

// NodeResponse wraps node mutation responses.
type NodeResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Node    *KnowledgeNode `json:"node"`
}

// DeleteResponse is used by node and relation deletes.
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// RelationListResponse wraps relation rows.
type RelationListResponse struct {
	Items []KnowledgeRelation `json:"items"`
	Total int                 `json:"total"`
}

// RelationResponse wraps relation mutation responses.
type RelationResponse struct {
	Success  bool               `json:"success"`
	Message  string             `json:"message"`
	Relation *KnowledgeRelation `json:"relation"`
}

// Service implements admin knowledge management use cases.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService creates a knowledge service.
func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("knowledge repository is nil")
	}
	return &Service{repo: repo, now: time.Now}, nil
}

// GetStats returns knowledge graph counters.
func (s *Service) GetStats(ctx context.Context) (Stats, error) {
	return s.repo.Stats(ctx)
}

// GetChapters returns distinct non-empty chapter names.
func (s *Service) GetChapters(ctx context.Context) ([]string, error) {
	return s.repo.DistinctChapters(ctx)
}

// ListNodes returns a paginated node list.
func (s *Service) ListNodes(ctx context.Context, filter NodeFilter) (NodeListResponse, error) {
	filter = normalizeNodeFilter(filter)
	if filter.NodeType != "" && !validNodeType(filter.NodeType) {
		return NodeListResponse{}, badRequest("无效的节点类型: " + filter.NodeType)
	}
	total, err := s.repo.CountNodes(ctx, filter)
	if err != nil {
		return NodeListResponse{}, err
	}
	items, err := s.repo.ListNodes(ctx, filter)
	if err != nil {
		return NodeListResponse{}, err
	}
	return NodeListResponse{
		Items:      items,
		Total:      total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalPages: numutil.TotalPages(total, filter.PageSize),
	}, nil
}

// GetAllNodesSimple returns all compact node rows.
func (s *Service) GetAllNodesSimple(ctx context.Context) ([]SimpleNode, error) {
	return s.repo.ListAllSimpleNodes(ctx)
}

// GetNode returns one knowledge node.
func (s *Service) GetNode(ctx context.Context, nodeID string) (KnowledgeNode, error) {
	node, ok, err := s.repo.GetNode(ctx, strings.TrimSpace(nodeID))
	if err != nil {
		return KnowledgeNode{}, err
	}
	if !ok {
		return KnowledgeNode{}, Error{Kind: ErrNotFound, Message: "知识节点不存在"}
	}
	return node, nil
}

// CreateNode creates a new knowledge node.
func (s *Service) CreateNode(ctx context.Context, input NodeInput) (NodeResponse, error) {
	input = normalizeNodeInput(input)
	if input.Name == "" {
		return NodeResponse{}, badRequest("name 不能为空")
	}
	if !validNodeType(input.NodeType) {
		return NodeResponse{}, badRequest("无效的节点类型: " + input.NodeType)
	}
	node, err := s.repo.CreateNode(ctx, input, s.now())
	if err != nil {
		return NodeResponse{}, err
	}
	return NodeResponse{Success: true, Message: "创建成功", Node: &node}, nil
}

// UpdateNode updates a knowledge node.
func (s *Service) UpdateNode(ctx context.Context, nodeID string, update NodeUpdate) (NodeResponse, error) {
	update = normalizeNodeUpdate(update)
	if !nodeUpdateHasFields(update) {
		return NodeResponse{}, badRequest("没有需要更新的字段")
	}
	if update.NodeType != nil && !validNodeType(*update.NodeType) {
		return NodeResponse{}, badRequest("无效的节点类型: " + *update.NodeType)
	}
	node, ok, err := s.repo.UpdateNode(ctx, strings.TrimSpace(nodeID), update, s.now())
	if err != nil {
		return NodeResponse{}, err
	}
	if !ok {
		return NodeResponse{}, Error{Kind: ErrNotFound, Message: "知识节点不存在"}
	}
	return NodeResponse{Success: true, Message: "更新成功", Node: &node}, nil
}

// DeleteNode deletes a node and its relations.
func (s *Service) DeleteNode(ctx context.Context, nodeID string) (DeleteResponse, error) {
	ok, err := s.repo.DeleteNode(ctx, strings.TrimSpace(nodeID))
	if err != nil {
		return DeleteResponse{}, err
	}
	if !ok {
		return DeleteResponse{}, Error{Kind: ErrNotFound, Message: "知识节点不存在"}
	}
	return DeleteResponse{Success: true, Message: "删除成功"}, nil
}

// ListRelations returns relations, optionally filtered by a node.
func (s *Service) ListRelations(ctx context.Context, nodeID string) (RelationListResponse, error) {
	items, err := s.repo.ListRelations(ctx, strings.TrimSpace(nodeID))
	if err != nil {
		return RelationListResponse{}, err
	}
	return RelationListResponse{Items: items, Total: len(items)}, nil
}

// CreateRelation creates a knowledge relation.
func (s *Service) CreateRelation(ctx context.Context, input RelationInput) (RelationResponse, error) {
	input = normalizeRelationInput(input)
	if err := s.validateRelationInput(ctx, input); err != nil {
		return RelationResponse{}, err
	}
	relation, err := s.repo.CreateRelation(ctx, input, s.now())
	if err != nil {
		return RelationResponse{}, err
	}
	return RelationResponse{Success: true, Message: "创建成功", Relation: &relation}, nil
}

// UpdateRelation updates a knowledge relation.
func (s *Service) UpdateRelation(ctx context.Context, relationID string, update RelationUpdate) (RelationResponse, error) {
	update = normalizeRelationUpdate(update)
	if !relationUpdateHasFields(update) {
		return RelationResponse{}, badRequest("没有需要更新的字段")
	}
	if update.RelationType != nil && !validRelationType(*update.RelationType) {
		return RelationResponse{}, badRequest("无效的关系类型: " + *update.RelationType)
	}
	relation, ok, err := s.repo.UpdateRelation(ctx, strings.TrimSpace(relationID), update)
	if err != nil {
		return RelationResponse{}, err
	}
	if !ok {
		return RelationResponse{}, Error{Kind: ErrNotFound, Message: "知识关系不存在"}
	}
	return RelationResponse{Success: true, Message: "更新成功", Relation: &relation}, nil
}

// DeleteRelation deletes a knowledge relation.
func (s *Service) DeleteRelation(ctx context.Context, relationID string) (DeleteResponse, error) {
	ok, err := s.repo.DeleteRelation(ctx, strings.TrimSpace(relationID))
	if err != nil {
		return DeleteResponse{}, err
	}
	if !ok {
		return DeleteResponse{}, Error{Kind: ErrNotFound, Message: "知识关系不存在"}
	}
	return DeleteResponse{Success: true, Message: "删除成功"}, nil
}

func (s *Service) validateRelationInput(ctx context.Context, input RelationInput) error {
	if input.SourceID == "" {
		return badRequest("源节点不存在")
	}
	if input.TargetID == "" {
		return badRequest("目标节点不存在")
	}
	if input.SourceID == input.TargetID {
		return badRequest("源节点和目标节点不能相同")
	}
	if !validRelationType(input.RelationType) {
		return badRequest("无效的关系类型: " + input.RelationType)
	}
	sourceOK, err := s.repo.NodeExists(ctx, input.SourceID)
	if err != nil {
		return err
	}
	if !sourceOK {
		return badRequest("源节点不存在")
	}
	targetOK, err := s.repo.NodeExists(ctx, input.TargetID)
	if err != nil {
		return err
	}
	if !targetOK {
		return badRequest("目标节点不存在")
	}
	return nil
}

func normalizeNodeFilter(filter NodeFilter) NodeFilter {
	filter.Chapter = strings.TrimSpace(filter.Chapter)
	filter.NodeType = strings.ToLower(strings.TrimSpace(filter.NodeType))
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

func normalizeNodeInput(input NodeInput) NodeInput {
	input.Name = strings.TrimSpace(input.Name)
	input.NodeType = strings.ToLower(strings.TrimSpace(input.NodeType))
	input.Description = strings.TrimSpace(input.Description)
	if input.Tags == nil {
		input.Tags = []string{}
	}
	if input.Difficulty < 0 {
		input.Difficulty = 0
	}
	if input.Difficulty > 1 {
		input.Difficulty = 1
	}
	return input
}

func normalizeNodeUpdate(update NodeUpdate) NodeUpdate {
	if update.Name != nil {
		value := strings.TrimSpace(*update.Name)
		update.Name = &value
	}
	if update.NodeType != nil {
		value := strings.ToLower(strings.TrimSpace(*update.NodeType))
		update.NodeType = &value
	}
	if update.Description != nil {
		value := strings.TrimSpace(*update.Description)
		update.Description = &value
	}
	if update.Tags != nil && *update.Tags == nil {
		values := []string{}
		update.Tags = &values
	}
	return update
}

func normalizeRelationInput(input RelationInput) RelationInput {
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.RelationType = strings.ToLower(strings.TrimSpace(input.RelationType))
	if input.Weight < 0 {
		input.Weight = 0
	}
	if input.Weight > 1 {
		input.Weight = 1
	}
	return input
}

func normalizeRelationUpdate(update RelationUpdate) RelationUpdate {
	if update.RelationType != nil {
		value := strings.ToLower(strings.TrimSpace(*update.RelationType))
		update.RelationType = &value
	}
	return update
}

func nodeUpdateHasFields(update NodeUpdate) bool {
	return update.Name != nil ||
		update.NameEn != nil ||
		update.NodeType != nil ||
		update.Description != nil ||
		update.Chapter != nil ||
		update.Section != nil ||
		update.Difficulty != nil ||
		update.LatexFormula != nil ||
		update.Tags != nil
}

func relationUpdateHasFields(update RelationUpdate) bool {
	return update.RelationType != nil || update.Weight != nil || update.Description != nil
}

func validNodeType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "concept", "theorem", "method", "problem", "misconception", "resource":
		return true
	default:
		return false
	}
}

func validRelationType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "has_prerequisite", "is_a_special_case_of", "used_in", "prone_to_error", "related_to":
		return true
	default:
		return false
	}
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}
