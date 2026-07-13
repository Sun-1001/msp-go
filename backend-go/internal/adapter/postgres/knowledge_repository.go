package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	knowledgeapp "mathstudy/backend-go/internal/application/knowledge"
)

const knowledgeNodeColumns = `
	id,
	name,
	name_en,
	node_type::text,
	description,
	chapter,
	section,
	difficulty,
	latex_formula,
	tags,
	created_at,
	updated_at`

const knowledgeRelationColumns = `
	kr.id,
	kr.source_id,
	kr.target_id,
	src.name,
	dst.name,
	kr.relation_type::text,
	kr.weight,
	kr.description,
	kr.created_at`

// KnowledgeRepository persists admin knowledge graph data in PostgreSQL.
type KnowledgeRepository struct {
	Repository
}

// NewKnowledgeRepository creates a PostgreSQL-backed knowledge repository.
func NewKnowledgeRepository(db Querier) (KnowledgeRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return KnowledgeRepository{}, err
	}
	return KnowledgeRepository{Repository: base}, nil
}

// Stats returns aggregate knowledge graph counters.
func (r KnowledgeRepository) Stats(ctx context.Context) (knowledgeapp.Stats, error) {
	var stats knowledgeapp.Stats
	stats.TypeDistribution = map[string]int{}
	if err := r.DB().QueryRow(ctx, `SELECT count(id)::int FROM public.knowledge_nodes`).Scan(&stats.TotalNodes); err != nil {
		return knowledgeapp.Stats{}, err
	}
	if err := r.DB().QueryRow(ctx, `SELECT count(id)::int FROM public.knowledge_relations`).Scan(&stats.TotalRelations); err != nil {
		return knowledgeapp.Stats{}, err
	}
	if err := r.DB().QueryRow(ctx, `
		SELECT count(DISTINCT chapter)::int
		FROM public.knowledge_nodes
		WHERE chapter IS NOT NULL AND chapter <> ''`).Scan(&stats.ChaptersCount); err != nil {
		return knowledgeapp.Stats{}, err
	}
	rows, err := r.DB().Query(ctx, `
		SELECT node_type::text, count(id)::int
		FROM public.knowledge_nodes
		GROUP BY node_type
		ORDER BY node_type`)
	if err != nil {
		return knowledgeapp.Stats{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var nodeType string
		var count int
		if err := rows.Scan(&nodeType, &count); err != nil {
			return knowledgeapp.Stats{}, err
		}
		stats.TypeDistribution[nodeTypeFromDB(nodeType)] = count
	}
	return stats, rows.Err()
}

// DistinctChapters returns sorted non-empty chapter names.
func (r KnowledgeRepository) DistinctChapters(ctx context.Context) ([]string, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT DISTINCT chapter
		FROM public.knowledge_nodes
		WHERE chapter IS NOT NULL AND chapter <> ''
		ORDER BY chapter`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringColumn(rows)
}

// CountNodes counts nodes matching filters.
func (r KnowledgeRepository) CountNodes(ctx context.Context, filter knowledgeapp.NodeFilter) (int, error) {
	where, args := knowledgeNodeWhere(filter)
	var count int
	err := r.DB().QueryRow(ctx, `
		SELECT count(id)::int
		FROM public.knowledge_nodes
		WHERE `+where,
		args...,
	).Scan(&count)
	return count, err
}

// ListNodes returns a filtered node page.
func (r KnowledgeRepository) ListNodes(ctx context.Context, filter knowledgeapp.NodeFilter) ([]knowledgeapp.KnowledgeNode, error) {
	where, args := knowledgeNodeWhere(filter)
	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)
	limitPlaceholder := fmt.Sprintf("$%d", len(args)-1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(args))
	rows, err := r.DB().Query(ctx, `
		SELECT `+knowledgeNodeColumns+`
		FROM public.knowledge_nodes
		WHERE `+where+`
		ORDER BY created_at
		LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []knowledgeapp.KnowledgeNode{}
	for rows.Next() {
		node, err := scanAdminKnowledgeNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

// ListAllSimpleNodes returns compact node rows for relation selectors.
func (r KnowledgeRepository) ListAllSimpleNodes(ctx context.Context) ([]knowledgeapp.SimpleNode, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT id, name, chapter, node_type::text
		FROM public.knowledge_nodes
		ORDER BY chapter NULLS LAST, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []knowledgeapp.SimpleNode{}
	for rows.Next() {
		var node knowledgeapp.SimpleNode
		var chapter pgtype.Text
		var nodeType string
		if err := rows.Scan(&node.ID, &node.Name, &chapter, &nodeType); err != nil {
			return nil, err
		}
		node.Chapter = textPtr(chapter)
		value := nodeTypeFromDB(nodeType)
		node.NodeType = &value
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

// GetNode returns one knowledge node by ID.
func (r KnowledgeRepository) GetNode(ctx context.Context, nodeID string) (knowledgeapp.KnowledgeNode, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT `+knowledgeNodeColumns+`
		FROM public.knowledge_nodes
		WHERE id = $1`,
		nodeID,
	)
	node, err := scanAdminKnowledgeNode(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return knowledgeapp.KnowledgeNode{}, false, nil
		}
		return knowledgeapp.KnowledgeNode{}, false, err
	}
	return node, true, nil
}

// CreateNode inserts a knowledge node.
func (r KnowledgeRepository) CreateNode(ctx context.Context, input knowledgeapp.NodeInput, now time.Time) (knowledgeapp.KnowledgeNode, error) {
	nodeID, err := newUUID()
	if err != nil {
		return knowledgeapp.KnowledgeNode{}, err
	}
	tags, err := json.Marshal(input.Tags)
	if err != nil {
		return knowledgeapp.KnowledgeNode{}, err
	}
	row := r.DB().QueryRow(ctx, `
		INSERT INTO public.knowledge_nodes (
			id,
			name,
			name_en,
			node_type,
			description,
			chapter,
			section,
			difficulty,
			latex_formula,
			tags,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4::public.nodetype, $5, $6, $7, $8, $9, $10::json, $11, $11)
		RETURNING `+knowledgeNodeColumns,
		nodeID,
		input.Name,
		input.NameEn,
		nodeTypeToDB(input.NodeType),
		input.Description,
		input.Chapter,
		input.Section,
		input.Difficulty,
		input.LatexFormula,
		string(tags),
		now,
	)
	return scanAdminKnowledgeNode(row)
}

// UpdateNode updates one knowledge node.
func (r KnowledgeRepository) UpdateNode(ctx context.Context, nodeID string, update knowledgeapp.NodeUpdate, now time.Time) (knowledgeapp.KnowledgeNode, bool, error) {
	sets := []string{}
	args := []any{nodeID}
	addSet := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if update.Name != nil {
		addSet("name", *update.Name)
	}
	if update.NameEn != nil {
		addSet("name_en", *update.NameEn)
	}
	if update.NodeType != nil {
		addSet("node_type", nodeTypeToDB(*update.NodeType))
		sets[len(sets)-1] += "::public.nodetype"
	}
	if update.Description != nil {
		addSet("description", *update.Description)
	}
	if update.Chapter != nil {
		addSet("chapter", *update.Chapter)
	}
	if update.Section != nil {
		addSet("section", *update.Section)
	}
	if update.Difficulty != nil {
		addSet("difficulty", *update.Difficulty)
	}
	if update.LatexFormula != nil {
		addSet("latex_formula", *update.LatexFormula)
	}
	if update.Tags != nil {
		tags, err := json.Marshal(*update.Tags)
		if err != nil {
			return knowledgeapp.KnowledgeNode{}, false, err
		}
		addSet("tags", string(tags))
		sets[len(sets)-1] += "::json"
	}
	args = append(args, now)
	sets = append(sets, fmt.Sprintf("updated_at = $%d", len(args)))
	row := r.DB().QueryRow(ctx, `
		UPDATE public.knowledge_nodes
		SET `+strings.Join(sets, ", ")+`
		WHERE id = $1
		RETURNING `+knowledgeNodeColumns,
		args...,
	)
	node, err := scanAdminKnowledgeNode(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return knowledgeapp.KnowledgeNode{}, false, nil
		}
		return knowledgeapp.KnowledgeNode{}, false, err
	}
	return node, true, nil
}

// DeleteNode deletes a node and all adjacent relations.
func (r KnowledgeRepository) DeleteNode(ctx context.Context, nodeID string) (bool, error) {
	deleted := false
	err := r.withTx(ctx, func(tx KnowledgeRepository) error {
		if _, err := tx.DB().Exec(ctx, `
			DELETE FROM public.knowledge_relations
			WHERE source_id = $1 OR target_id = $1`,
			nodeID,
		); err != nil {
			return err
		}
		tag, err := tx.DB().Exec(ctx, `
			DELETE FROM public.knowledge_nodes
			WHERE id = $1`,
			nodeID,
		)
		if err != nil {
			return err
		}
		deleted = tag.RowsAffected() > 0
		return nil
	})
	return deleted, err
}

// NodeExists reports whether a node exists.
func (r KnowledgeRepository) NodeExists(ctx context.Context, nodeID string) (bool, error) {
	return r.Exists(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM public.knowledge_nodes
			WHERE id = $1
		)`,
		nodeID,
	)
}

// ListRelations returns relations with source and target node names.
func (r KnowledgeRepository) ListRelations(ctx context.Context, nodeID string) ([]knowledgeapp.KnowledgeRelation, error) {
	query := `
		SELECT ` + knowledgeRelationColumns + `
		FROM public.knowledge_relations kr
		LEFT JOIN public.knowledge_nodes src ON src.id = kr.source_id
		LEFT JOIN public.knowledge_nodes dst ON dst.id = kr.target_id`
	args := []any{}
	if strings.TrimSpace(nodeID) != "" {
		args = append(args, nodeID)
		query += `
		WHERE kr.source_id = $1 OR kr.target_id = $1`
	}
	query += `
		ORDER BY kr.created_at`
	rows, err := r.DB().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	relations := []knowledgeapp.KnowledgeRelation{}
	for rows.Next() {
		relation, err := scanAdminKnowledgeRelation(rows)
		if err != nil {
			return nil, err
		}
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

// CreateRelation inserts a knowledge relation.
func (r KnowledgeRepository) CreateRelation(ctx context.Context, input knowledgeapp.RelationInput, now time.Time) (knowledgeapp.KnowledgeRelation, error) {
	relationID, err := newUUID()
	if err != nil {
		return knowledgeapp.KnowledgeRelation{}, err
	}
	if _, err := r.DB().Exec(ctx, `
		INSERT INTO public.knowledge_relations (
			id,
			source_id,
			target_id,
			relation_type,
			weight,
			description,
			created_at
		)
		VALUES ($1, $2, $3, $4::public.relationtype, $5, $6, $7)`,
		relationID,
		input.SourceID,
		input.TargetID,
		relationTypeToDB(input.RelationType),
		input.Weight,
		input.Description,
		now,
	); err != nil {
		return knowledgeapp.KnowledgeRelation{}, err
	}
	relation, ok, err := r.getRelationByID(ctx, relationID)
	if err != nil {
		return knowledgeapp.KnowledgeRelation{}, err
	}
	if !ok {
		return knowledgeapp.KnowledgeRelation{}, pgx.ErrNoRows
	}
	return relation, nil
}

// UpdateRelation updates one knowledge relation.
func (r KnowledgeRepository) UpdateRelation(ctx context.Context, relationID string, update knowledgeapp.RelationUpdate) (knowledgeapp.KnowledgeRelation, bool, error) {
	sets := []string{}
	args := []any{relationID}
	addSet := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if update.RelationType != nil {
		addSet("relation_type", relationTypeToDB(*update.RelationType))
		sets[len(sets)-1] += "::public.relationtype"
	}
	if update.Weight != nil {
		addSet("weight", *update.Weight)
	}
	if update.Description != nil {
		addSet("description", *update.Description)
	}
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.knowledge_relations
		SET `+strings.Join(sets, ", ")+`
		WHERE id = $1`,
		args...,
	)
	if err != nil {
		return knowledgeapp.KnowledgeRelation{}, false, err
	}
	if tag.RowsAffected() == 0 {
		return knowledgeapp.KnowledgeRelation{}, false, nil
	}
	relation, ok, err := r.getRelationByID(ctx, relationID)
	return relation, ok, err
}

// DeleteRelation deletes one relation.
func (r KnowledgeRepository) DeleteRelation(ctx context.Context, relationID string) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		DELETE FROM public.knowledge_relations
		WHERE id = $1`,
		relationID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r KnowledgeRepository) getRelationByID(ctx context.Context, relationID string) (knowledgeapp.KnowledgeRelation, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT `+knowledgeRelationColumns+`
		FROM public.knowledge_relations kr
		LEFT JOIN public.knowledge_nodes src ON src.id = kr.source_id
		LEFT JOIN public.knowledge_nodes dst ON dst.id = kr.target_id
		WHERE kr.id = $1`,
		relationID,
	)
	relation, err := scanAdminKnowledgeRelation(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return knowledgeapp.KnowledgeRelation{}, false, nil
		}
		return knowledgeapp.KnowledgeRelation{}, false, err
	}
	return relation, true, nil
}

func (r KnowledgeRepository) withTx(ctx context.Context, fn func(KnowledgeRepository) error) error {
	return withRepositoryTx(ctx, "knowledge", r.Repository, func(base Repository) KnowledgeRepository {
		return KnowledgeRepository{Repository: base}
	}, fn)
}

func knowledgeNodeWhere(filter knowledgeapp.NodeFilter) (string, []any) {
	conditions := []string{"TRUE"}
	args := []any{}
	if strings.TrimSpace(filter.Chapter) != "" {
		args = append(args, filter.Chapter)
		conditions = append(conditions, fmt.Sprintf("chapter = $%d", len(args)))
	}
	if strings.TrimSpace(filter.NodeType) != "" {
		args = append(args, nodeTypeToDB(filter.NodeType))
		conditions = append(conditions, fmt.Sprintf("node_type = $%d::public.nodetype", len(args)))
	}
	if strings.TrimSpace(filter.Search) != "" {
		args = append(args, filter.Search)
		placeholder := fmt.Sprintf("$%d", len(args))
		conditions = append(conditions, "(name ILIKE '%' || "+placeholder+" || '%' OR name_en ILIKE '%' || "+placeholder+" || '%' OR description ILIKE '%' || "+placeholder+" || '%')")
	}
	return strings.Join(conditions, " AND "), args
}

func scanAdminKnowledgeNode(scanner rowScanner) (knowledgeapp.KnowledgeNode, error) {
	var node knowledgeapp.KnowledgeNode
	var nameEn pgtype.Text
	var chapter pgtype.Text
	var section pgtype.Text
	var latexFormula pgtype.Text
	var nodeType string
	var tagsRaw []byte
	if err := scanner.Scan(
		&node.ID,
		&node.Name,
		&nameEn,
		&nodeType,
		&node.Description,
		&chapter,
		&section,
		&node.Difficulty,
		&latexFormula,
		&tagsRaw,
		&node.CreatedAt,
		&node.UpdatedAt,
	); err != nil {
		return knowledgeapp.KnowledgeNode{}, err
	}
	node.NodeType = nodeTypeFromDB(nodeType)
	node.NameEn = textPtr(nameEn)
	node.Chapter = textPtr(chapter)
	node.Section = textPtr(section)
	node.LatexFormula = textPtr(latexFormula)
	tags, err := decodeStringSlice(tagsRaw)
	if err != nil {
		return knowledgeapp.KnowledgeNode{}, fmt.Errorf("decode knowledge node tags: %w", err)
	}
	node.Tags = tags
	return node, nil
}

func scanAdminKnowledgeRelation(scanner rowScanner) (knowledgeapp.KnowledgeRelation, error) {
	var relation knowledgeapp.KnowledgeRelation
	var sourceName pgtype.Text
	var targetName pgtype.Text
	var relationType string
	var description pgtype.Text
	if err := scanner.Scan(
		&relation.ID,
		&relation.SourceID,
		&relation.TargetID,
		&sourceName,
		&targetName,
		&relationType,
		&relation.Weight,
		&description,
		&relation.CreatedAt,
	); err != nil {
		return knowledgeapp.KnowledgeRelation{}, err
	}
	relation.SourceName = textPtr(sourceName)
	relation.TargetName = textPtr(targetName)
	relation.RelationType = relationTypeFromDB(relationType)
	relation.Description = textPtr(description)
	return relation, nil
}

func nodeTypeToDB(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func nodeTypeFromDB(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func relationTypeToDB(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func relationTypeFromDB(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
