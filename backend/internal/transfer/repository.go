package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ExportBranchMarkdown(ctx context.Context, projectID, branchID string) (MarkdownDocument, error) {
	var projectName, branchName string
	err := r.db.QueryRow(ctx, `SELECT p.name, b.name FROM branches b JOIN projects p ON p.id=b.project_id WHERE b.id=$1 AND b.project_id=$2`, branchID, projectID).Scan(&projectName, &branchName)
	if err != nil {
		return MarkdownDocument{}, normalizeNotFound(err)
	}
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE lineage AS (
			SELECT id, base_branch_id, fork_from_block_id, 0 AS depth FROM branches WHERE id=$1 AND project_id=$2
			UNION ALL
			SELECT parent.id, parent.base_branch_id, child.fork_from_block_id, child.depth+1
			FROM branches parent JOIN lineage child ON child.base_branch_id=parent.id
		)
		SELECT b.title, b.type, COALESCE(r.content, ''), COALESCE(r.content_format, 'markdown')
		FROM lineage l
		JOIN blocks b ON b.branch_id=l.id
		LEFT JOIN block_revisions r ON r.id=b.current_revision_id
		WHERE l.depth=0 OR b.order_index <= COALESCE((SELECT order_index FROM blocks WHERE id=l.fork_from_block_id), b.order_index)
		ORDER BY l.depth DESC, b.order_index, b.created_at
	`, branchID, projectID)
	if err != nil {
		return MarkdownDocument{}, err
	}
	defer rows.Close()
	sections, err := scanMarkdownSections(rows)
	if err != nil {
		return MarkdownDocument{}, err
	}
	return MarkdownDocument{
		Filename: safeFilename(projectName + "-" + branchName + ".md"),
		Content:  buildMarkdown(projectName+" · "+branchName, sections),
	}, nil
}

func (r *Repository) ExportChapterMarkdown(ctx context.Context, projectID, chapterID string) (MarkdownDocument, error) {
	var projectName, chapterName, branchID string
	var orderIndex int
	err := r.db.QueryRow(ctx, `
		SELECT p.name, COALESCE(b.title, '未命名章节'), b.branch_id::text, b.order_index
		FROM blocks b JOIN projects p ON p.id=b.project_id
		WHERE b.id=$1 AND b.project_id=$2 AND b.type='chapter' AND b.branch_id IS NOT NULL
	`, chapterID, projectID).Scan(&projectName, &chapterName, &branchID, &orderIndex)
	if err != nil {
		return MarkdownDocument{}, normalizeNotFound(err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT b.title, b.type, COALESCE(r.content, ''), COALESCE(r.content_format, 'markdown')
		FROM blocks b LEFT JOIN block_revisions r ON r.id=b.current_revision_id
		WHERE b.project_id=$1 AND b.branch_id=$2 AND b.order_index >= $3
		  AND b.order_index < COALESCE((
			SELECT MIN(next.order_index) FROM blocks next
			WHERE next.branch_id=$2 AND next.type='chapter' AND next.order_index>$3
		  ), 2147483647)
		ORDER BY b.order_index, b.created_at
	`, projectID, branchID, orderIndex)
	if err != nil {
		return MarkdownDocument{}, err
	}
	defer rows.Close()
	sections, err := scanMarkdownSections(rows)
	if err != nil {
		return MarkdownDocument{}, err
	}
	return MarkdownDocument{
		Filename: safeFilename(projectName + "-" + chapterName + ".md"),
		Content:  buildMarkdown(chapterName, sections),
	}, nil
}

type markdownRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanMarkdownSections(rows markdownRows) ([]markdownSection, error) {
	result := make([]markdownSection, 0)
	for rows.Next() {
		var section markdownSection
		if err := rows.Scan(&section.Title, &section.Type, &section.Content, &section.Format); err != nil {
			return nil, err
		}
		result = append(result, section)
	}
	return result, rows.Err()
}

var backupTables = []string{
	"projects", "prompt_templates", "branches", "blocks",
	"generation_runs", "block_revisions", "graph_edges", "canon_entities",
	"memory_chunks", "summary_snapshots", "character_states", "foreshadowings",
	"timeline_events", "llm_conversations", "llm_messages",
}

func (r *Repository) Backup(ctx context.Context, projectID string) (Backup, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1)`, projectID).Scan(&exists); err != nil {
		return Backup{}, err
	}
	if !exists {
		return Backup{}, ErrNotFound
	}
	tables := make(map[string]json.RawMessage, len(backupTables))
	for _, table := range backupTables {
		query := backupQuery(table)
		var raw []byte
		if err := r.db.QueryRow(ctx, query, projectID).Scan(&raw); err != nil {
			return Backup{}, err
		}
		tables[table] = raw
	}
	return Backup{Version: BackupVersion, ProjectID: projectID, Tables: tables}, nil
}

func backupQuery(table string) string {
	switch table {
	case "projects":
		return `SELECT jsonb_agg(to_jsonb(t)) FROM projects t WHERE id=$1`
	case "block_revisions":
		return `SELECT COALESCE(jsonb_agg(to_jsonb(t)), '[]') FROM block_revisions t JOIN blocks b ON b.id=t.block_id WHERE b.project_id=$1`
	case "llm_messages":
		return `SELECT COALESCE(jsonb_agg(to_jsonb(t)), '[]') FROM llm_messages t JOIN llm_conversations c ON c.id=t.conversation_id WHERE c.project_id=$1`
	case "canon_entities", "memory_chunks":
		return fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(t)-'embedding'), '[]') FROM %s t WHERE project_id=$1`, table)
	default:
		return fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(t)), '[]') FROM %s t WHERE project_id=$1`, table)
	}
}

func (r *Repository) Import(ctx context.Context, backup Backup) error {
	if backup.Version != BackupVersion || strings.TrimSpace(backup.ProjectID) == "" {
		return ErrInvalidExport
	}
	for _, table := range backupTables {
		if !json.Valid(backup.Tables[table]) {
			return ErrInvalidExport
		}
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1)`, backup.ProjectID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrImportConflict
	}
	insertOrder := []string{"projects", "prompt_templates", "branches", "blocks", "generation_runs", "block_revisions", "graph_edges", "canon_entities", "memory_chunks", "summary_snapshots", "character_states", "foreshadowings", "timeline_events", "llm_conversations", "llm_messages"}
	for _, table := range insertOrder {
		raw := backup.Tables[table]
		query := importInsertQuery(table)
		if _, err := tx.Exec(ctx, query, raw); err != nil {
			return fmt.Errorf("import %s: %w", table, err)
		}
	}
	for _, query := range restoreReferenceQueries {
		if _, err := tx.Exec(ctx, query, backup.Tables[referenceTable(query)]); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func importInsertQuery(table string) string {
	source := fmt.Sprintf(`jsonb_populate_recordset(NULL::%s, $1::jsonb)`, table)
	switch table {
	case "projects":
		return `INSERT INTO projects SELECT (r).* FROM (SELECT jsonb_populate_record(NULL::projects, (item-'default_model_profile_id')) AS r FROM jsonb_array_elements($1::jsonb) item) s`
	case "branches":
		return `INSERT INTO branches SELECT (r).* FROM (SELECT jsonb_populate_record(NULL::branches, item-'base_branch_id'-'fork_from_block_id'-'fork_from_revision_id') AS r FROM jsonb_array_elements($1::jsonb) item) s`
	case "blocks":
		return `INSERT INTO blocks SELECT (r).* FROM (SELECT jsonb_populate_record(NULL::blocks, item-'current_revision_id'-'parent_block_id') AS r FROM jsonb_array_elements($1::jsonb) item) s`
	case "generation_runs":
		return `INSERT INTO generation_runs SELECT (r).* FROM (SELECT jsonb_populate_record(NULL::generation_runs, item-'output_revision_id') AS r FROM jsonb_array_elements($1::jsonb) item) s`
	case "block_revisions":
		return `INSERT INTO block_revisions SELECT (r).* FROM (SELECT jsonb_populate_record(NULL::block_revisions, item-'parent_revision_id'-'generation_run_id') AS r FROM jsonb_array_elements($1::jsonb) item) s`
	default:
		return fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`, table, source)
	}
}

var restoreReferenceQueries = []string{
	`UPDATE branches dst SET base_branch_id=src.base_branch_id, fork_from_block_id=src.fork_from_block_id, fork_from_revision_id=src.fork_from_revision_id FROM jsonb_populate_recordset(NULL::branches, $1::jsonb) src WHERE dst.id=src.id`,
	`UPDATE blocks dst SET current_revision_id=src.current_revision_id, parent_block_id=src.parent_block_id FROM jsonb_populate_recordset(NULL::blocks, $1::jsonb) src WHERE dst.id=src.id`,
	`UPDATE generation_runs dst SET output_revision_id=src.output_revision_id FROM jsonb_populate_recordset(NULL::generation_runs, $1::jsonb) src WHERE dst.id=src.id`,
	`UPDATE block_revisions dst SET parent_revision_id=src.parent_revision_id, generation_run_id=src.generation_run_id FROM jsonb_populate_recordset(NULL::block_revisions, $1::jsonb) src WHERE dst.id=src.id`,
}

func referenceTable(query string) string {
	for _, table := range []string{"branches", "blocks", "generation_runs", "block_revisions"} {
		if strings.HasPrefix(query, "UPDATE "+table+" ") {
			return table
		}
	}
	return ""
}

func normalizeNotFound(err error) error {
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	return err
}
