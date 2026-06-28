package story

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ListCharacterStates(ctx context.Context, projectID, characterID string) ([]CharacterState, error) {
	query := `SELECT id::text, project_id::text, character_id::text, block_id::text,
		state_key, state_value, notes, occurred_at, metadata, created_at, updated_at
		FROM character_states WHERE project_id=$1`
	args := []any{projectID}
	if characterID != "" {
		query += ` AND character_id=$2`
		args = append(args, characterID)
	}
	query += ` ORDER BY COALESCE(occurred_at, ''), created_at`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]CharacterState, 0)
	for rows.Next() {
		item, err := scanCharacterState(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) CreateCharacterState(ctx context.Context, projectID string, input CharacterStateInput) (CharacterState, error) {
	input.CharacterID, input.StateKey = normalizeText(input.CharacterID), normalizeText(input.StateKey)
	if input.CharacterID == "" || input.StateKey == "" {
		return CharacterState{}, ErrInvalidRecord
	}
	var err error
	if input.StateValue, err = normalizeJSON(input.StateValue); err != nil {
		return CharacterState{}, err
	}
	if input.Metadata, err = normalizeJSON(input.Metadata); err != nil {
		return CharacterState{}, err
	}
	return scanCharacterState(r.db.QueryRow(ctx, `INSERT INTO character_states
		(project_id, character_id, block_id, state_key, state_value, notes, occurred_at, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text, project_id::text, character_id::text, block_id::text,
		state_key, state_value, notes, occurred_at, metadata, created_at, updated_at`,
		projectID, input.CharacterID, input.BlockID, input.StateKey, input.StateValue,
		input.Notes, input.OccurredAt, input.Metadata))
}

func (r *Repository) UpdateCharacterState(ctx context.Context, id string, input CharacterStateInput) (CharacterState, error) {
	input.CharacterID, input.StateKey = normalizeText(input.CharacterID), normalizeText(input.StateKey)
	if input.CharacterID == "" || input.StateKey == "" {
		return CharacterState{}, ErrInvalidRecord
	}
	var err error
	if input.StateValue, err = normalizeJSON(input.StateValue); err != nil {
		return CharacterState{}, err
	}
	if input.Metadata, err = normalizeJSON(input.Metadata); err != nil {
		return CharacterState{}, err
	}
	item, err := scanCharacterState(r.db.QueryRow(ctx, `UPDATE character_states SET
		character_id=$2, block_id=$3, state_key=$4, state_value=$5, notes=$6,
		occurred_at=$7, metadata=$8 WHERE id=$1
		RETURNING id::text, project_id::text, character_id::text, block_id::text,
		state_key, state_value, notes, occurred_at, metadata, created_at, updated_at`,
		id, input.CharacterID, input.BlockID, input.StateKey, input.StateValue,
		input.Notes, input.OccurredAt, input.Metadata))
	return item, normalizeNotFound(err)
}

func (r *Repository) ListForeshadowings(ctx context.Context, projectID, status string) ([]Foreshadowing, error) {
	query := `SELECT id::text, project_id::text, title, description, status,
		planted_block_id::text, resolved_block_id::text, metadata, created_at, updated_at
		FROM foreshadowings WHERE project_id=$1`
	args := []any{projectID}
	if status != "" {
		if !validForeshadowingStatus(status) {
			return nil, ErrInvalidRecord
		}
		query += ` AND status=$2`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Foreshadowing, 0)
	for rows.Next() {
		item, err := scanForeshadowing(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) CreateForeshadowing(ctx context.Context, projectID string, input ForeshadowingInput) (Foreshadowing, error) {
	input.Title, input.Status = normalizeText(input.Title), normalizeText(input.Status)
	if input.Status == "" {
		input.Status = "planted"
	}
	if input.Title == "" || !validForeshadowingStatus(input.Status) {
		return Foreshadowing{}, ErrInvalidRecord
	}
	var err error
	if input.Metadata, err = normalizeJSON(input.Metadata); err != nil {
		return Foreshadowing{}, err
	}
	return scanForeshadowing(r.db.QueryRow(ctx, `INSERT INTO foreshadowings
		(project_id, title, description, status, planted_block_id, resolved_block_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id::text, project_id::text, title, description, status,
		planted_block_id::text, resolved_block_id::text, metadata, created_at, updated_at`,
		projectID, input.Title, input.Description, input.Status, input.PlantedBlockID,
		input.ResolvedBlockID, input.Metadata))
}

func (r *Repository) UpdateForeshadowing(ctx context.Context, id string, input ForeshadowingInput) (Foreshadowing, error) {
	input.Title, input.Status = normalizeText(input.Title), normalizeText(input.Status)
	if input.Title == "" || !validForeshadowingStatus(input.Status) {
		return Foreshadowing{}, ErrInvalidRecord
	}
	metadata, err := normalizeJSON(input.Metadata)
	if err != nil {
		return Foreshadowing{}, err
	}
	item, err := scanForeshadowing(r.db.QueryRow(ctx, `UPDATE foreshadowings SET
		title=$2, description=$3, status=$4, planted_block_id=$5, resolved_block_id=$6, metadata=$7
		WHERE id=$1 RETURNING id::text, project_id::text, title, description, status,
		planted_block_id::text, resolved_block_id::text, metadata, created_at, updated_at`,
		id, input.Title, input.Description, input.Status, input.PlantedBlockID,
		input.ResolvedBlockID, metadata))
	return item, normalizeNotFound(err)
}

func (r *Repository) ListTimelineEvents(ctx context.Context, projectID string) ([]TimelineEvent, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text, project_id::text, title, description,
		event_time, sort_order, block_id::text, canon_entity_id::text, metadata, created_at, updated_at
		FROM timeline_events WHERE project_id=$1 ORDER BY sort_order, created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]TimelineEvent, 0)
	for rows.Next() {
		item, err := scanTimelineEvent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) CreateTimelineEvent(ctx context.Context, projectID string, input TimelineEventInput) (TimelineEvent, error) {
	input.Title = normalizeText(input.Title)
	if input.Title == "" {
		return TimelineEvent{}, ErrInvalidRecord
	}
	metadata, err := normalizeJSON(input.Metadata)
	if err != nil {
		return TimelineEvent{}, err
	}
	return scanTimelineEvent(r.db.QueryRow(ctx, `INSERT INTO timeline_events
		(project_id,title,description,event_time,sort_order,block_id,canon_entity_id,metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text, project_id::text, title, description, event_time, sort_order,
		block_id::text, canon_entity_id::text, metadata, created_at, updated_at`,
		projectID, input.Title, input.Description, input.EventTime, input.SortOrder,
		input.BlockID, input.CanonEntityID, metadata))
}

func (r *Repository) UpdateTimelineEvent(ctx context.Context, id string, input TimelineEventInput) (TimelineEvent, error) {
	input.Title = normalizeText(input.Title)
	if input.Title == "" {
		return TimelineEvent{}, ErrInvalidRecord
	}
	metadata, err := normalizeJSON(input.Metadata)
	if err != nil {
		return TimelineEvent{}, err
	}
	item, err := scanTimelineEvent(r.db.QueryRow(ctx, `UPDATE timeline_events SET
		title=$2, description=$3, event_time=$4, sort_order=$5, block_id=$6,
		canon_entity_id=$7, metadata=$8 WHERE id=$1
		RETURNING id::text, project_id::text, title, description, event_time, sort_order,
		block_id::text, canon_entity_id::text, metadata, created_at, updated_at`,
		id, input.Title, input.Description, input.EventTime, input.SortOrder,
		input.BlockID, input.CanonEntityID, metadata))
	return item, normalizeNotFound(err)
}

func (r *Repository) Delete(ctx context.Context, table, id string) error {
	if table != "character_states" && table != "foreshadowings" && table != "timeline_events" {
		return ErrInvalidRecord
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM `+table+` WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanCharacterState(s scanner) (CharacterState, error) {
	var item CharacterState
	var blockID, notes, occurredAt sql.NullString
	err := s.Scan(&item.ID, &item.ProjectID, &item.CharacterID, &blockID, &item.StateKey,
		&item.StateValue, &notes, &occurredAt, &item.Metadata, &item.CreatedAt, &item.UpdatedAt)
	if blockID.Valid {
		item.BlockID = &blockID.String
	}
	if notes.Valid {
		item.Notes = &notes.String
	}
	if occurredAt.Valid {
		item.OccurredAt = &occurredAt.String
	}
	return item, normalizeNotFound(err)
}

func scanForeshadowing(s scanner) (Foreshadowing, error) {
	var item Foreshadowing
	var description, planted, resolved sql.NullString
	err := s.Scan(&item.ID, &item.ProjectID, &item.Title, &description, &item.Status,
		&planted, &resolved, &item.Metadata, &item.CreatedAt, &item.UpdatedAt)
	if description.Valid {
		item.Description = &description.String
	}
	if planted.Valid {
		item.PlantedBlockID = &planted.String
	}
	if resolved.Valid {
		item.ResolvedBlockID = &resolved.String
	}
	return item, normalizeNotFound(err)
}

func scanTimelineEvent(s scanner) (TimelineEvent, error) {
	var item TimelineEvent
	var description, eventTime, blockID, canonID sql.NullString
	err := s.Scan(&item.ID, &item.ProjectID, &item.Title, &description, &eventTime,
		&item.SortOrder, &blockID, &canonID, &item.Metadata, &item.CreatedAt, &item.UpdatedAt)
	if description.Valid {
		item.Description = &description.String
	}
	if eventTime.Valid {
		item.EventTime = &eventTime.String
	}
	if blockID.Valid {
		item.BlockID = &blockID.String
	}
	if canonID.Valid {
		item.CanonEntityID = &canonID.String
	}
	return item, normalizeNotFound(err)
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
