package postgres

import (
	"context"
	progressapp "mathstudy/backend/internal/application/progress"
)

// ListCourseConcepts serves exercise selectors and learning paths only.
func (r ProgressRepository) ListCourseConcepts(ctx context.Context) ([]progressapp.CourseConcept, error) {
	rows, err := r.DB().Query(ctx, `SELECT id,name,description,chapter,difficulty FROM public.course_concepts WHERE learning_enabled ORDER BY chapter NULLS LAST,name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	concepts := []progressapp.CourseConcept{}
	for rows.Next() {
		var concept progressapp.CourseConcept
		if err := rows.Scan(&concept.ID, &concept.Name, &concept.Description, &concept.Chapter, &concept.Difficulty); err != nil {
			return nil, err
		}
		concepts = append(concepts, concept)
	}
	return concepts, rows.Err()
}
func (r ProgressRepository) ListPrerequisites(ctx context.Context) ([]progressapp.Prerequisite, error) {
	rows, err := r.DB().Query(ctx, `SELECT concept_id,prerequisite_id FROM public.course_prerequisites ORDER BY concept_id,prerequisite_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []progressapp.Prerequisite{}
	for rows.Next() {
		var item progressapp.Prerequisite
		if err := rows.Scan(&item.SourceID, &item.TargetID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
