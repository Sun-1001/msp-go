package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	bktapp "mathstudy/backend-go/internal/application/bkt"
)

// BKTRepository persists admin BKT parameter data in PostgreSQL.
type BKTRepository struct {
	Repository
}

// NewBKTRepository creates a PostgreSQL-backed BKT repository.
func NewBKTRepository(db Querier) (BKTRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return BKTRepository{}, err
	}
	return BKTRepository{Repository: base}, nil
}

// ListParams returns a parameter page and total count.
func (r BKTRepository) ListParams(ctx context.Context, offset int, limit int) ([]bktapp.Param, int, error) {
	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT count(concept_id)::int
		FROM public.concept_bkt_params`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB().Query(ctx, `
		SELECT concept_id, p_l0, p_t, p_g, p_s
		FROM public.concept_bkt_params
		ORDER BY concept_id
		LIMIT $1 OFFSET $2`,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	params := []bktapp.Param{}
	for rows.Next() {
		param, err := scanBKTParam(rows)
		if err != nil {
			return nil, 0, err
		}
		params = append(params, param)
	}
	return params, total, rows.Err()
}

// UpdateParam updates one concept parameter row.
func (r BKTRepository) UpdateParam(ctx context.Context, conceptID string, update bktapp.Update, now time.Time) (bktapp.Param, bool, error) {
	row := r.DB().QueryRow(ctx, `
		UPDATE public.concept_bkt_params
		SET
			p_l0 = COALESCE($2::double precision, p_l0),
			p_t = COALESCE($3::double precision, p_t),
			p_g = COALESCE($4::double precision, p_g),
			p_s = COALESCE($5::double precision, p_s),
			updated_at = $6
		WHERE concept_id = $1
		RETURNING concept_id, p_l0, p_t, p_g, p_s`,
		conceptID,
		optionalFloat(update.PL0),
		optionalFloat(update.PT),
		optionalFloat(update.PG),
		optionalFloat(update.PS),
		now,
	)
	param, err := scanBKTParam(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return bktapp.Param{}, false, nil
		}
		return bktapp.Param{}, false, err
	}
	return param, true, nil
}

// ResetParam resets one concept parameter row to defaults.
func (r BKTRepository) ResetParam(ctx context.Context, conceptID string, defaults bktapp.Param, now time.Time) (bktapp.Param, bool, error) {
	row := r.DB().QueryRow(ctx, `
		UPDATE public.concept_bkt_params
		SET p_l0 = $2, p_t = $3, p_g = $4, p_s = $5, updated_at = $6
		WHERE concept_id = $1
		RETURNING concept_id, p_l0, p_t, p_g, p_s`,
		conceptID,
		defaults.PL0,
		defaults.PT,
		defaults.PG,
		defaults.PS,
		now,
	)
	param, err := scanBKTParam(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return bktapp.Param{}, false, nil
		}
		return bktapp.Param{}, false, err
	}
	return param, true, nil
}

// SeedDefaultParams creates default parameter rows for all knowledge nodes missing them.
func (r BKTRepository) SeedDefaultParams(ctx context.Context, defaults bktapp.Param, now time.Time) (int, error) {
	tag, err := r.DB().Exec(ctx, `
		INSERT INTO public.concept_bkt_params (
			concept_id,
			p_l0,
			p_t,
			p_g,
			p_s,
			created_at,
			updated_at
		)
		SELECT n.id, $1, $2, $3, $4, $5, $5
		FROM public.knowledge_nodes n
		WHERE NOT EXISTS (
			SELECT 1
			FROM public.concept_bkt_params p
			WHERE p.concept_id = n.id
		)
		ON CONFLICT (concept_id) DO NOTHING`,
		defaults.PL0,
		defaults.PT,
		defaults.PG,
		defaults.PS,
		now,
	)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func scanBKTParam(scanner rowScanner) (bktapp.Param, error) {
	var param bktapp.Param
	if err := scanner.Scan(&param.ConceptID, &param.PL0, &param.PT, &param.PG, &param.PS); err != nil {
		return bktapp.Param{}, err
	}
	return param, nil
}

func optionalFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}
