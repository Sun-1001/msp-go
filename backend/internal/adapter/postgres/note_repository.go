package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	noteapp "mathstudy/backend/internal/application/note"
)

type NoteRepository struct{ Repository }

func NewNoteRepository(db Querier) (NoteRepository, error) {
	base, err := NewRepository(db)
	return NoteRepository{base}, err
}

const noteColumns = `id, owner_id, title, body, material_id, source_info, revision, created_at, updated_at`

func scanNote(row pgx.Row) (noteapp.Note, error) {
	var note noteapp.Note
	err := row.Scan(&note.ID, &note.OwnerID, &note.Title, &note.Body, &note.MaterialID, &note.SourceInfo, &note.Revision, &note.CreatedAt, &note.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = noteapp.ErrNotFound
	}
	return note, err
}
func (r NoteRepository) Get(ctx context.Context, owner, id string) (noteapp.Note, error) {
	return scanNote(r.DB().QueryRow(ctx, `SELECT `+noteColumns+` FROM public.notes WHERE owner_id=$1 AND id=$2`, owner, id))
}
func (r NoteRepository) List(ctx context.Context, owner, search string) ([]noteapp.Summary, error) {
	rows, err := r.DB().Query(ctx, `SELECT id, title, updated_at FROM public.notes WHERE owner_id=$1 AND ($2='' OR strpos(lower(title),lower($2))>0 OR strpos(lower(body),lower($2))>0) ORDER BY updated_at DESC,id`, owner, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []noteapp.Summary{}
	for rows.Next() {
		var item noteapp.Summary
		if err := rows.Scan(&item.ID, &item.Title, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r NoteRepository) transaction(ctx context.Context, owner string, fn func(NoteRepository) error) error {
	return withRepositoryTx(ctx, "notes", r.Repository, func(base Repository) NoteRepository { return NoteRepository{base} }, func(tx NoteRepository) error {
		if _, err := tx.DB().Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('personal-notes:' || $1,0))`, owner); err != nil {
			return err
		}
		return fn(tx)
	})
}
func (r NoteRepository) Save(ctx context.Context, owner, id string, input noteapp.Input, create bool) (noteapp.Note, error) {
	var saved noteapp.Note
	err := r.transaction(ctx, owner, func(tx NoteRepository) error {
		var err error
		saved, err = tx.save(ctx, owner, id, input, create)
		return err
	})
	return saved, err
}
func (r NoteRepository) save(ctx context.Context, owner, id string, input noteapp.Input, create bool) (noteapp.Note, error) {
	targets := noteapp.Targets(input.Body)
	if err := noteapp.Validate(input); err != nil {
		return noteapp.Note{}, err
	}
	if !create {
		existing, err := r.Get(ctx, owner, id)
		if err != nil {
			return noteapp.Note{}, err
		}
		if existing.Revision != input.Revision {
			return noteapp.Note{}, noteapp.ErrConflict
		}
	} else {
		var count int
		if err := r.DB().QueryRow(ctx, `SELECT count(*) FROM public.notes WHERE owner_id=$1`, owner).Scan(&count); err != nil {
			return noteapp.Note{}, err
		}
		if count >= noteapp.MaxNotes {
			return noteapp.Note{}, noteapp.ErrLimit
		}
	}
	if input.MaterialID != nil {
		if _, err := r.GetMaterial(ctx, owner, *input.MaterialID); err != nil {
			return noteapp.Note{}, err
		}
	}
	var count int
	if err := r.DB().QueryRow(ctx, `SELECT count(*) FROM public.notes WHERE owner_id=$1 AND id=ANY($2::varchar[]) AND id<>$3`, owner, targets, id).Scan(&count); err != nil {
		return noteapp.Note{}, err
	}
	if count != len(targets) {
		return noteapp.Note{}, noteapp.ErrInvalid
	}
	now := time.Now().UTC()
	var saved noteapp.Note
	var err error
	if create {
		saved, err = scanNote(r.DB().QueryRow(ctx, `INSERT INTO public.notes(id,owner_id,title,body,material_id,source_info,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7) RETURNING `+noteColumns, id, owner, input.Title, input.Body, input.MaterialID, input.SourceInfo, now))
	} else {
		saved, err = scanNote(r.DB().QueryRow(ctx, `UPDATE public.notes SET title=$3,body=$4,material_id=$5,source_info=$6,revision=revision+1,updated_at=$7 WHERE id=$1 AND owner_id=$2 RETURNING `+noteColumns, id, owner, input.Title, input.Body, input.MaterialID, input.SourceInfo, now))
	}
	if err != nil {
		return noteapp.Note{}, err
	}
	if _, err = r.DB().Exec(ctx, `DELETE FROM public.note_links WHERE owner_id=$1 AND source_id=$2 AND NOT(target_id=ANY($3::varchar[]))`, owner, id, targets); err != nil {
		return noteapp.Note{}, err
	}
	if _, err = r.DB().Exec(ctx, `INSERT INTO public.note_links(owner_id,source_id,target_id,created_at) SELECT $1,$2,unnest($3::varchar[]),$4 ON CONFLICT DO NOTHING`, owner, id, targets, now); err != nil {
		return noteapp.Note{}, err
	}
	return saved, nil
}
func (r NoteRepository) Delete(ctx context.Context, owner, id string, revision int64) error {
	return r.transaction(ctx, owner, func(tx NoteRepository) error {
		existing, err := tx.Get(ctx, owner, id)
		if err != nil {
			return err
		}
		if existing.Revision != revision {
			return noteapp.ErrConflict
		}
		rows, err := tx.DB().Query(ctx, `SELECT n.id,n.body FROM public.notes n JOIN public.note_links l ON l.owner_id=n.owner_id AND l.source_id=n.id WHERE l.owner_id=$1 AND l.target_id=$2`, owner, id)
		if err != nil {
			return err
		}
		type inbound struct{ id, body string }
		items := []inbound{}
		for rows.Next() {
			var item inbound
			if err := rows.Scan(&item.id, &item.body); err != nil {
				rows.Close()
				return err
			}
			items = append(items, item)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, item := range items {
			if _, err := tx.DB().Exec(ctx, `UPDATE public.notes SET body=$3,revision=revision+1,updated_at=$4 WHERE owner_id=$1 AND id=$2`, owner, item.id, noteapp.RemoveLink(item.body, id), time.Now().UTC()); err != nil {
				return err
			}
		}
		_, err = tx.DB().Exec(ctx, `DELETE FROM public.notes WHERE owner_id=$1 AND id=$2`, owner, id)
		return err
	})
}
func (r NoteRepository) ChangeLink(ctx context.Context, owner, source, target string, revision int64, add bool) (noteapp.Note, error) {
	var saved noteapp.Note
	err := r.transaction(ctx, owner, func(tx NoteRepository) error {
		n, err := tx.Get(ctx, owner, source)
		if err != nil {
			return err
		}
		if n.Revision != revision {
			return noteapp.ErrConflict
		}
		t, err := tx.Get(ctx, owner, target)
		if err != nil {
			return err
		}
		body := noteapp.RemoveLink(n.Body, target)
		if add {
			body = noteapp.AppendLink(n.Body, target, t.Title)
		}
		saved, err = tx.save(ctx, owner, source, noteapp.Input{Title: n.Title, Body: body, MaterialID: n.MaterialID, SourceInfo: n.SourceInfo, Revision: revision}, false)
		return err
	})
	return saved, err
}
func (r NoteRepository) Network(ctx context.Context, owner, id string) (noteapp.Network, error) {
	// One statement keeps nodes and edges in the same MVCC snapshot.
	var data []byte
	var exists bool
	err := r.DB().QueryRow(ctx, `WITH visible AS (
	      SELECT id,title,updated_at AT TIME ZONE 'UTC' AS updated_at FROM public.notes WHERE owner_id=$1 AND ($2='' OR id=$2 OR id IN (
        SELECT target_id FROM public.note_links WHERE owner_id=$1 AND source_id=$2
        UNION SELECT source_id FROM public.note_links WHERE owner_id=$1 AND target_id=$2)))
      SELECT ($2='' OR EXISTS(SELECT 1 FROM public.notes WHERE owner_id=$1 AND id=$2)),json_build_object(
        'nodes',coalesce((SELECT json_agg(v ORDER BY v.id) FROM visible v),'[]'),
	        'links',coalesce((SELECT json_agg(l ORDER BY l.source_id,l.target_id) FROM (SELECT owner_id,source_id,target_id,source_type,created_at AT TIME ZONE 'UTC' AS created_at FROM public.note_links WHERE owner_id=$1 AND source_id IN(SELECT id FROM visible) AND target_id IN(SELECT id FROM visible)) l),'[]'))`, owner, id).Scan(&exists, &data)
	if err != nil {
		return noteapp.Network{}, err
	}
	if !exists {
		return noteapp.Network{}, noteapp.ErrNotFound
	}
	var network noteapp.Network
	err = json.Unmarshal(data, &network)
	return network, err
}

const materialColumns = `id,owner_id,filename,content_type,byte_size,object_uri,storage_key,created_at`

func scanMaterial(row pgx.Row) (noteapp.Material, error) {
	var m noteapp.Material
	err := row.Scan(&m.ID, &m.OwnerID, &m.Filename, &m.ContentType, &m.ByteSize, &m.URI, &m.StorageKey, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = noteapp.ErrNotFound
	}
	return m, err
}
func (r NoteRepository) SaveMaterial(ctx context.Context, m noteapp.Material) (noteapp.Material, error) {
	return scanMaterial(r.DB().QueryRow(ctx, `INSERT INTO public.note_materials(id,owner_id,filename,content_type,byte_size,object_uri,storage_key) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+materialColumns, m.ID, m.OwnerID, m.Filename, m.ContentType, m.ByteSize, m.URI, m.StorageKey))
}
func (r NoteRepository) GetMaterial(ctx context.Context, owner, id string) (noteapp.Material, error) {
	return scanMaterial(r.DB().QueryRow(ctx, `SELECT `+materialColumns+` FROM public.note_materials WHERE owner_id=$1 AND id=$2`, owner, id))
}
func (r NoteRepository) ListMaterials(ctx context.Context, owner string) ([]noteapp.Material, error) {
	rows, err := r.DB().Query(ctx, `SELECT `+materialColumns+` FROM public.note_materials WHERE owner_id=$1 ORDER BY created_at DESC,id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []noteapp.Material{}
	for rows.Next() {
		m, err := scanMaterial(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
