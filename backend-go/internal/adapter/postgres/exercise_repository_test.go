package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	exerciseapp "mathstudy/backend-go/internal/application/exercise"
)

type knowledgeConceptRow struct {
	err error
}

func (r knowledgeConceptRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*string) = "concept-1"
	*dest[1].(*string) = "函数极限"
	*dest[2].(*string) = "研究函数在某一点附近的变化趋势"
	*dest[3].(*pgtype.Text) = pgtype.Text{String: "第一章", Valid: true}
	return nil
}

type generatedExerciseRow struct{}

func (generatedExerciseRow) Scan(dest ...any) error {
	*dest[0].(*string) = "exercise-1"
	*dest[1].(*pgtype.Text) = pgtype.Text{}
	*dest[2].(*pgtype.Text) = pgtype.Text{String: "student-1", Valid: true}
	*dest[3].(*string) = "PUBLISHED"
	*dest[4].(*string) = "函数极限练习"
	*dest[5].(*string) = "求给定函数的极限。"
	*dest[6].(*float64) = 0.6
	*dest[7].(*[]byte) = []byte(`["concept-1"]`)
	*dest[8].(*[]byte) = []byte(`{"answer":"2","answer_type":"text","type":"multiple_choice","options":["1","2","3","4"],"hints":["先化简"],"solution_steps":["约分"],"estimated_time_seconds":180,"knowledge_point_names":["函数极限"]}`)
	return nil
}

type exerciseProfileRow struct{}

func (exerciseProfileRow) Scan(dest ...any) error {
	*dest[0].(*[]byte) = []byte(`{}`)
	*dest[1].(*[]byte) = []byte(`{}`)
	*dest[2].(*float64) = 0.5
	*dest[3].(*float64) = 1
	*dest[4].(*int) = 0
	*dest[5].(*int) = 0
	return nil
}

type recordingExerciseQuerier struct {
	querySQL  string
	queryArgs []any
	execSQL   string
	execArgs  []any
	row       pgx.Row
}

func (q *recordingExerciseQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execSQL = sql
	q.execArgs = append([]any(nil), args...)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (*recordingExerciseQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *recordingExerciseQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.querySQL = sql
	q.queryArgs = append([]any(nil), args...)
	return q.row
}

func TestExerciseRepositoryGetKnowledgeConcept(t *testing.T) {
	repo, err := NewExerciseRepository(fakeQuerier{row: knowledgeConceptRow{}})
	if err != nil {
		t.Fatalf("NewExerciseRepository() error = %v", err)
	}

	concept, ok, err := repo.GetKnowledgeConcept(context.Background(), "concept-1")
	if err != nil {
		t.Fatalf("GetKnowledgeConcept() error = %v", err)
	}
	if !ok || concept.ID != "concept-1" || concept.Name != "函数极限" || concept.Chapter != "第一章" {
		t.Fatalf("GetKnowledgeConcept() = %#v, %t", concept, ok)
	}
}

func TestExerciseRepositoryGetKnowledgeConceptNotFound(t *testing.T) {
	repo, err := NewExerciseRepository(fakeQuerier{row: knowledgeConceptRow{err: pgx.ErrNoRows}})
	if err != nil {
		t.Fatalf("NewExerciseRepository() error = %v", err)
	}

	concept, ok, err := repo.GetKnowledgeConcept(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetKnowledgeConcept() error = %v", err)
	}
	if ok || concept != (exerciseapp.KnowledgeConcept{}) {
		t.Fatalf("GetKnowledgeConcept() = %#v, %t, want not found", concept, ok)
	}
}

func TestExerciseRepositoryGetExerciseForUpdateUsesSharedRowLock(t *testing.T) {
	querier := &recordingExerciseQuerier{row: generatedExerciseRow{}}
	repo, err := NewExerciseRepository(querier)
	if err != nil {
		t.Fatalf("NewExerciseRepository() error = %v", err)
	}

	exercise, ok, err := repo.GetExerciseForUpdate(context.Background(), "exercise-1")
	if err != nil {
		t.Fatalf("GetExerciseForUpdate() error = %v", err)
	}
	if !ok || exercise.ID != "exercise-1" {
		t.Fatalf("GetExerciseForUpdate() = %#v, %t", exercise, ok)
	}
	if !strings.Contains(querier.querySQL, "FOR SHARE") {
		t.Fatalf("GetExerciseForUpdate() SQL does not hold a shared lock: %s", querier.querySQL)
	}
	if len(querier.queryArgs) != 1 || querier.queryArgs[0] != "exercise-1" {
		t.Fatalf("GetExerciseForUpdate() args = %#v", querier.queryArgs)
	}
}

func TestExerciseRepositoryCreateGeneratedExercisePersistsStudentOwnershipAndMeta(t *testing.T) {
	querier := &recordingExerciseQuerier{row: generatedExerciseRow{}}
	repo, err := NewExerciseRepository(querier)
	if err != nil {
		t.Fatalf("NewExerciseRepository() error = %v", err)
	}
	now := time.Date(2026, time.July, 12, 10, 30, 0, 0, time.UTC)
	generated := exerciseapp.GeneratedQuestion{
		Title:                "函数极限练习",
		Body:                 "求给定函数的极限。",
		Type:                 "multiple_choice",
		Answer:               "2",
		AnswerType:           "text",
		Options:              []string{"1", "2", "3", "4"},
		Hints:                []string{"先化简"},
		SolutionSteps:        []string{"约分"},
		EstimatedTimeSeconds: 180,
		Difficulty:           0.6,
		ConceptIDs:           []string{"concept-1"},
		KnowledgePointNames:  []string{"函数极限"},
	}

	exercise, err := repo.CreateGeneratedExercise(context.Background(), "student-1", generated, now)
	if err != nil {
		t.Fatalf("CreateGeneratedExercise() error = %v", err)
	}
	if exercise.OwnerTeacherID != "" || exercise.GeneratedByStudentID != "student-1" || exercise.Status != "PUBLISHED" {
		t.Fatalf("CreateGeneratedExercise() = %#v", exercise)
	}
	if !strings.Contains(querier.querySQL, "owner_teacher_id") || !strings.Contains(querier.querySQL, "generated_by_student_id") || !strings.Contains(querier.querySQL, "'PROBLEM'::public.contenttype") || !strings.Contains(querier.querySQL, "RETURNING") {
		t.Fatalf("insert SQL = %s", querier.querySQL)
	}
	if len(querier.queryArgs) != 8 || querier.queryArgs[1] != "student-1" || querier.queryArgs[7] != now {
		t.Fatalf("insert args = %#v", querier.queryArgs)
	}

	var conceptIDs []string
	if err := json.Unmarshal([]byte(querier.queryArgs[5].(string)), &conceptIDs); err != nil {
		t.Fatalf("decode stored concept IDs: %v", err)
	}
	if len(conceptIDs) != 1 || conceptIDs[0] != "concept-1" {
		t.Fatalf("stored concept IDs = %#v", conceptIDs)
	}
	var meta struct {
		Answer               string   `json:"answer"`
		AnswerType           string   `json:"answer_type"`
		Type                 string   `json:"type"`
		Options              []string `json:"options"`
		Hints                []string `json:"hints"`
		SolutionSteps        []string `json:"solution_steps"`
		EstimatedTimeSeconds int      `json:"estimated_time_seconds"`
		KnowledgePointNames  []string `json:"knowledge_point_names"`
	}
	if err := json.Unmarshal([]byte(querier.queryArgs[6].(string)), &meta); err != nil {
		t.Fatalf("decode stored meta: %v", err)
	}
	if meta.Answer != generated.Answer || meta.AnswerType != generated.AnswerType || meta.Type != generated.Type ||
		len(meta.Options) != 4 || len(meta.Hints) != 1 || len(meta.SolutionSteps) != 1 ||
		meta.EstimatedTimeSeconds != generated.EstimatedTimeSeconds || len(meta.KnowledgePointNames) != 1 {
		t.Fatalf("stored meta = %#v", meta)
	}
}

func TestExerciseRepositoryCreateProfileUsesTrackingDefaults(t *testing.T) {
	querier := &recordingExerciseQuerier{row: exerciseProfileRow{}}
	repo, err := NewExerciseRepository(querier)
	if err != nil {
		t.Fatalf("NewExerciseRepository() error = %v", err)
	}
	now := time.Date(2026, time.July, 12, 10, 30, 0, 0, time.UTC)

	profile, err := repo.CreateProfile(context.Background(), "student-1", now)
	if err != nil {
		t.Fatalf("CreateProfile() error = %v", err)
	}
	if profile.PreferredDifficulty != 0.5 || profile.LearningPace != 1 || profile.TotalExercises != 0 || profile.CorrectCount != 0 {
		t.Fatalf("CreateProfile() = %#v", profile)
	}
	for _, fragment := range []string{
		"INSERT INTO public.student_profiles",
		"'{}'::json",
		"0.5, 1.0, 0, 0, 0, '[]'::json",
		"ON CONFLICT (student_id)",
		"RETURNING",
	} {
		if !strings.Contains(querier.querySQL, fragment) {
			t.Fatalf("CreateProfile() SQL missing %q: %s", fragment, querier.querySQL)
		}
	}
	if len(querier.queryArgs) != 3 || querier.queryArgs[1] != "student-1" || querier.queryArgs[2] != now {
		t.Fatalf("CreateProfile() args = %#v", querier.queryArgs)
	}
}

func TestRecentClassContentIDsQueryExcludesStudentGeneratedExercises(t *testing.T) {
	for _, fragment := range []string{
		"JOIN public.contents c ON c.id = ca.content_id",
		"c.generated_by_student_id IS NULL",
		"ORDER BY ca.started_at DESC",
	} {
		if !strings.Contains(recentClassContentIDsSQL, fragment) {
			t.Fatalf("recentClassContentIDsSQL missing %q: %s", fragment, recentClassContentIDsSQL)
		}
	}
}

func TestUpdateSessionAfterSubmitOnlyClearsMatchingCurrentExercise(t *testing.T) {
	querier := &recordingExerciseQuerier{}
	repo, err := NewExerciseRepository(querier)
	if err != nil {
		t.Fatalf("NewExerciseRepository() error = %v", err)
	}

	err = repo.UpdateSessionAfterSubmit(
		context.Background(),
		"session-1",
		"generated-1",
		[]string{"generated-1"},
	)
	if err != nil {
		t.Fatalf("UpdateSessionAfterSubmit() error = %v", err)
	}
	if !strings.Contains(querier.execSQL, "CASE WHEN current_content_id = $2 THEN NULL ELSE current_content_id END") {
		t.Fatalf("UpdateSessionAfterSubmit() SQL = %s", querier.execSQL)
	}
	if len(querier.execArgs) != 3 || querier.execArgs[1] != "generated-1" {
		t.Fatalf("UpdateSessionAfterSubmit() args = %#v", querier.execArgs)
	}
}
