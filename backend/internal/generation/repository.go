package generation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListConversations(ctx context.Context, blockID string) ([]Conversation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, project_id::text, block_id::text, title, created_at, updated_at
		FROM llm_conversations
		WHERE block_id = $1
		ORDER BY updated_at DESC
	`, blockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Conversation, 0)
	for rows.Next() {
		var item Conversation
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.BlockID, &item.Title, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {
	var item Conversation
	err := r.db.QueryRow(ctx, `
		SELECT id::text, project_id::text, block_id::text, title, created_at, updated_at
		FROM llm_conversations WHERE id = $1
	`, conversationID).Scan(
		&item.ID, &item.ProjectID, &item.BlockID, &item.Title, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, normalizeNotFound(err)
}

func (r *Repository) CreateConversation(ctx context.Context, blockID string, req CreateConversationRequest) (Conversation, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "新对话"
	}
	var item Conversation
	err := r.db.QueryRow(ctx, `
		INSERT INTO llm_conversations (project_id, block_id, title)
		SELECT project_id, id, $2 FROM blocks WHERE id = $1 AND project_id = $3
		RETURNING id::text, project_id::text, block_id::text, title, created_at, updated_at
	`, blockID, title, req.ProjectID).Scan(
		&item.ID, &item.ProjectID, &item.BlockID, &item.Title, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, normalizeNotFound(err)
}

func (r *Repository) UpdateConversation(ctx context.Context, conversationID string, title string) (Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Conversation{}, ErrInvalidGenerationRequest
	}
	var item Conversation
	err := r.db.QueryRow(ctx, `
		UPDATE llm_conversations SET title = $2, updated_at = now() WHERE id = $1
		RETURNING id::text, project_id::text, block_id::text, title, created_at, updated_at
	`, conversationID, title).Scan(
		&item.ID, &item.ProjectID, &item.BlockID, &item.Title, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, normalizeNotFound(err)
}

func (r *Repository) DeleteConversation(ctx context.Context, conversationID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM llm_conversations WHERE id = $1`, conversationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrGenerationResourceNotFound
	}
	return nil
}

func (r *Repository) ListConversationMessages(ctx context.Context, conversationID string) ([]ConversationMessage, error) {
	rows, err := r.db.Query(ctx, `
		SELECT message.id::text, message.conversation_id::text, message.role, message.content,
			message.generation_run_id::text, run.model, message.created_at, message.updated_at
		FROM llm_messages AS message
		LEFT JOIN generation_runs AS run ON run.id = message.generation_run_id
		WHERE message.conversation_id = $1
		ORDER BY message.created_at, message.id
	`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ConversationMessage, 0)
	for rows.Next() {
		message, err := scanConversationMessage(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, message)
	}
	return result, rows.Err()
}

func (r *Repository) AppendConversationMessage(ctx context.Context, conversationID string, role string, content string, runID *string) (ConversationMessage, error) {
	content = strings.TrimSpace(content)
	if (role != "user" && role != "assistant") || content == "" {
		return ConversationMessage{}, ErrInvalidGenerationRequest
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ConversationMessage{}, err
	}
	defer tx.Rollback(ctx)
	message, err := scanConversationMessage(tx.QueryRow(ctx, `
		INSERT INTO llm_messages (conversation_id, role, content, generation_run_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, conversation_id::text, role, content, generation_run_id::text,
			NULL::text, created_at, updated_at
	`, conversationID, role, content, nullableString(runID)))
	if err != nil {
		return ConversationMessage{}, normalizeNotFound(err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE llm_conversations
		SET
			title = CASE
				WHEN title = '新对话' AND $2 = 'user' THEN left($3, 40)
				ELSE title
			END,
			updated_at = now()
		WHERE id = $1
	`, conversationID, role, content); err != nil {
		return ConversationMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ConversationMessage{}, err
	}
	return message, nil
}

func (r *Repository) UpdateConversationMessage(ctx context.Context, messageID string, content string) (ConversationMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return ConversationMessage{}, ErrInvalidGenerationRequest
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ConversationMessage{}, err
	}
	defer tx.Rollback(ctx)
	var conversationID string
	if err := tx.QueryRow(ctx, `
		SELECT conversation_id::text FROM llm_messages WHERE id = $1
	`, messageID).Scan(&conversationID); err != nil {
		return ConversationMessage{}, normalizeNotFound(err)
	}
	message, err := scanConversationMessage(tx.QueryRow(ctx, `
		UPDATE llm_messages SET content = $2, updated_at = now() WHERE id = $1
		RETURNING id::text, conversation_id::text, role, content, generation_run_id::text,
			NULL::text, created_at, updated_at
	`, messageID, content))
	if err != nil {
		return ConversationMessage{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE llm_conversations SET updated_at = now() WHERE id = $1`, conversationID); err != nil {
		return ConversationMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ConversationMessage{}, err
	}
	return message, nil
}

func (r *Repository) ReplaceConversationAssistant(
	ctx context.Context,
	conversationID string,
	messageID string,
	content string,
	runID string,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE llm_messages
		SET content = $3, generation_run_id = $4, updated_at = now()
		WHERE id = $1 AND conversation_id = $2 AND role = 'assistant'
	`, messageID, conversationID, strings.TrimSpace(content), runID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrGenerationResourceNotFound
	}
	_, err = r.db.Exec(ctx, `UPDATE llm_conversations SET updated_at = now() WHERE id = $1`, conversationID)
	return err
}

type conversationMessageScanner interface {
	Scan(dest ...any) error
}

func scanConversationMessage(scanner conversationMessageScanner) (ConversationMessage, error) {
	var message ConversationMessage
	var runID sql.NullString
	var model sql.NullString
	err := scanner.Scan(
		&message.ID, &message.ConversationID, &message.Role, &message.Content,
		&runID, &model, &message.CreatedAt, &message.UpdatedAt,
	)
	if runID.Valid {
		message.GenerationRunID = &runID.String
	}
	if model.Valid {
		message.Model = &model.String
	}
	return message, err
}

func (r *Repository) GetModelProfile(ctx context.Context, projectID string, profileID string) (ModelProfile, error) {
	return r.getModelProfile(ctx, projectID, profileID, true)
}

func (r *Repository) GetModelProfileForPreview(ctx context.Context, projectID string, profileID string) (ModelProfile, error) {
	return r.getModelProfile(ctx, projectID, profileID, false)
}

func (r *Repository) getModelProfile(ctx context.Context, _ string, profileID string, resolveKey bool) (ModelProfile, error) {
	var profile ModelProfile
	var baseURL sql.NullString
	var apiKey sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT id::text, provider, model, base_url, api_key_ref, temperature, top_p, max_tokens, context_window
		FROM model_profiles
		WHERE id = $1 AND profile_type = 'llm'
	`, profileID).Scan(
		&profile.ID,
		&profile.Provider,
		&profile.Model,
		&baseURL,
		&apiKey,
		&profile.Temperature,
		&profile.TopP,
		&profile.MaxTokens,
		&profile.ContextWindow,
	)
	if err != nil {
		return ModelProfile{}, normalizeNotFound(err)
	}
	if baseURL.Valid {
		profile.BaseURL = &baseURL.String
	}
	if apiKey.Valid && resolveKey {
		resolved, err := resolveAPIKeyRef(apiKey.String)
		if err != nil {
			return ModelProfile{}, err
		}
		profile.APIKey = &resolved
	}
	return profile, nil
}

func resolveAPIKeyRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	envName, ok := strings.CutPrefix(ref, "env:")
	if !ok {
		if ref == "" {
			return "", fmt.Errorf("%w: api key is empty", ErrInvalidGenerationRequest)
		}
		return ref, nil
	}
	if strings.TrimSpace(envName) == "" {
		return "", fmt.Errorf("%w: api key environment variable name is empty", ErrInvalidGenerationRequest)
	}
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return "", fmt.Errorf("%w: api key environment variable %s is not configured", ErrInvalidGenerationRequest, envName)
	}
	return value, nil
}

func (r *Repository) GetBlockContext(ctx context.Context, projectID string, blockID string) (BlockContext, error) {
	var blockContext BlockContext
	var projectDescription sql.NullString
	var title sql.NullString
	var content sql.NullString
	var contentFormat sql.NullString
	var branchID sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT p.description, b.title, br.content, br.content_format, b.branch_id::text, b.order_index
		FROM blocks b
		JOIN projects p ON p.id = b.project_id
		LEFT JOIN block_revisions br ON br.id = b.current_revision_id
		WHERE b.id = $1 AND b.project_id = $2
	`, blockID, projectID).Scan(&projectDescription, &title, &content, &contentFormat, &branchID, &blockContext.OrderIndex)
	if err != nil {
		return BlockContext{}, normalizeNotFound(err)
	}
	if projectDescription.Valid {
		blockContext.ProjectDescription = &projectDescription.String
	}
	if title.Valid {
		blockContext.BlockTitle = &title.String
	}
	if content.Valid {
		blockContext.Content = content.String
	}
	if contentFormat.Valid {
		blockContext.ContentFormat = contentFormat.String
	}
	if branchID.Valid {
		blockContext.BranchID = &branchID.String
	}
	blockContext.CanonFacts, err = r.ListBlockCanonFacts(ctx, projectID, blockID)
	if err != nil {
		return BlockContext{}, err
	}
	return blockContext, nil
}

func (r *Repository) GetBlockMetadataContext(ctx context.Context, projectID string, blockID string) (BlockContext, error) {
	var blockContext BlockContext
	var projectDescription sql.NullString
	var title sql.NullString
	var branchID sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT p.description, b.title, b.branch_id::text, b.order_index
		FROM blocks b
		JOIN projects p ON p.id = b.project_id
		WHERE b.id = $1 AND b.project_id = $2
	`, blockID, projectID).Scan(&projectDescription, &title, &branchID, &blockContext.OrderIndex)
	if err != nil {
		return BlockContext{}, normalizeNotFound(err)
	}
	if projectDescription.Valid {
		blockContext.ProjectDescription = &projectDescription.String
	}
	if title.Valid {
		blockContext.BlockTitle = &title.String
	}
	if branchID.Valid {
		blockContext.BranchID = &branchID.String
	}
	blockContext.CanonFacts, err = r.ListBlockCanonFacts(ctx, projectID, blockID)
	if err != nil {
		return BlockContext{}, err
	}
	return blockContext, nil
}

func (r *Repository) ListBlockCanonFacts(ctx context.Context, projectID string, blockID string) ([]CanonFact, error) {
	rows, err := r.db.Query(ctx, `
		WITH block_metadata AS (
			SELECT metadata
			FROM blocks
			WHERE id = $2 AND project_id = $1
		),
		linked_ids AS (
			SELECT jsonb_array_elements_text(COALESCE(metadata->'character_ids', '[]'::jsonb)) AS id
			FROM block_metadata
			UNION
			SELECT metadata->>'location_id' AS id
			FROM block_metadata
			WHERE NULLIF(metadata->>'location_id', '') IS NOT NULL
		)
		SELECT
			id::text,
			type,
			name,
			aliases,
			description,
			attributes,
			importance,
			status
		FROM canon_entities
		WHERE project_id = $1
			AND status <> 'deprecated'
			AND (
				id::text IN (SELECT id FROM linked_ids)
				OR (type = 'rule' AND status = 'canon')
			)
		ORDER BY importance DESC, updated_at DESC
	`, projectID, blockID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facts := make([]CanonFact, 0)
	for rows.Next() {
		var fact CanonFact
		var description sql.NullString
		if err := rows.Scan(
			&fact.ID,
			&fact.Type,
			&fact.Name,
			&fact.Aliases,
			&description,
			&fact.Attributes,
			&fact.Importance,
			&fact.Status,
		); err != nil {
			return nil, err
		}
		if description.Valid {
			fact.Description = &description.String
		}
		if fact.Aliases == nil {
			fact.Aliases = []string{}
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}

func (r *Repository) GetCharacterCanonFact(ctx context.Context, projectID string, characterID string) (CanonFact, error) {
	var fact CanonFact
	var description sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT id::text, type, name, aliases, description, attributes, importance, status
		FROM canon_entities
		WHERE id = $1 AND project_id = $2 AND type = 'character'
	`, characterID, projectID).Scan(
		&fact.ID,
		&fact.Type,
		&fact.Name,
		&fact.Aliases,
		&description,
		&fact.Attributes,
		&fact.Importance,
		&fact.Status,
	)
	if err != nil {
		return CanonFact{}, normalizeNotFound(err)
	}
	if description.Valid {
		fact.Description = &description.String
	}
	if fact.Aliases == nil {
		fact.Aliases = []string{}
	}
	return fact, nil
}

func (r *Repository) ListRecentBlocks(ctx context.Context, projectID string, blockID string, limit int) ([]RecentBlockContext, error) {
	if limit == 0 {
		return []RecentBlockContext{}, nil
	}
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE story_walk AS (
			SELECT id, 0 AS depth, ARRAY[id] AS visited
			FROM blocks
			WHERE id = $2 AND project_id = $1
			UNION ALL
			SELECT edge.source_block_id, walk.depth + 1, walk.visited || edge.source_block_id
			FROM story_walk walk
			JOIN graph_edges edge ON edge.target_block_id = walk.id
			WHERE edge.project_id = $1
				AND edge.edge_type IN ('next', 'fork')
				AND NOT edge.source_block_id = ANY(walk.visited)
		),
		story_path AS (
			SELECT id, MIN(depth) AS depth
			FROM story_walk
			WHERE id <> $2
			GROUP BY id
		),
		candidates AS (
			SELECT b.id, b.title, revision.content, revision.content_format, b.order_index, path.depth
			FROM blocks b
			JOIN story_path path ON path.id = b.id
			JOIN block_revisions revision ON revision.id = b.current_revision_id
			WHERE b.project_id = $1
		)
		SELECT
			id::text,
			title,
			content,
			content_format,
			order_index
		FROM (
			SELECT *
			FROM candidates
			ORDER BY depth ASC, order_index DESC, id
			LIMIT NULLIF($3, -1)
		) selected
		ORDER BY depth DESC, order_index ASC, id
	`, projectID, blockID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := make([]RecentBlockContext, 0)
	for rows.Next() {
		var block RecentBlockContext
		var title sql.NullString
		if err := rows.Scan(&block.ID, &title, &block.Content, &block.ContentFormat, &block.OrderIndex); err != nil {
			return nil, err
		}
		if title.Valid {
			block.Title = &title.String
		}
		blocks = append(blocks, block)
	}
	return blocks, rows.Err()
}

func (r *Repository) ListMemoryForContext(ctx context.Context, projectID string, keywords []string, limit int) ([]MemoryContext, error) {
	if limit <= 0 || len(keywords) == 0 {
		return []MemoryContext{}, nil
	}
	patterns := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		patterns = append(patterns, "%"+keyword+"%")
	}
	if len(patterns) == 0 {
		return []MemoryContext{}, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			id::text,
			chunk_text,
			chunk_kind,
			tags
		FROM memory_chunks
		WHERE project_id = $1
			AND (
				chunk_text ILIKE ANY($2::text[])
				OR EXISTS (
					SELECT 1
					FROM unnest(tags) tag
					WHERE tag ILIKE ANY($2::text[])
				)
			)
		ORDER BY created_at DESC
		LIMIT $3
	`, projectID, patterns, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memories := make([]MemoryContext, 0)
	for rows.Next() {
		var memory MemoryContext
		if err := rows.Scan(&memory.ID, &memory.ChunkText, &memory.ChunkKind, &memory.Tags); err != nil {
			return nil, err
		}
		if memory.Tags == nil {
			memory.Tags = []string{}
		}
		memories = append(memories, memory)
	}
	return memories, rows.Err()
}

func (r *Repository) ListSummariesForContext(ctx context.Context, projectID string, blockID string, branchID *string) ([]SummaryContext, error) {
	if err := r.RefreshStaleSummaryStatuses(ctx, projectID); err != nil {
		return nil, err
	}
	args := []any{projectID, blockID}
	clauses := []string{`
		(target_type = 'chapter' AND target_id IN (
			SELECT candidate.id
			FROM blocks current
			JOIN blocks candidate ON candidate.project_id = current.project_id
			WHERE current.id = $2
				AND (
					candidate.id = current.id
					OR candidate.id = current.parent_block_id
				)
				AND candidate.type = 'chapter'
		))
	`}
	if branchID != nil && strings.TrimSpace(*branchID) != "" {
		args = append(args, *branchID)
		clauses = append(clauses, fmt.Sprintf("(target_type = 'branch' AND target_id = $%d)", len(args)))
	}

	query := fmt.Sprintf(`
		SELECT id::text, target_type, summary_text, token_count, status
		FROM (
			SELECT DISTINCT ON (target_type, target_id)
				id,
				target_type,
				target_id,
				summary_text,
				token_count,
				status,
				created_at
			FROM summary_snapshots
			WHERE project_id = $1
				AND status IN ('valid', 'stale')
				AND (%s)
			ORDER BY target_type, target_id, created_at DESC
		) latest
		ORDER BY status = 'valid' DESC, created_at DESC
		LIMIT 4
	`, strings.Join(clauses, " OR "))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]SummaryContext, 0)
	for rows.Next() {
		var summary SummaryContext
		if err := rows.Scan(&summary.ID, &summary.TargetType, &summary.SummaryText, &summary.TokenCount, &summary.Status); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (r *Repository) GetBlockSummarySource(ctx context.Context, projectID string, blockID string) (BlockSummarySource, error) {
	var source BlockSummarySource
	var title sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT
			CASE WHEN b.type = 'chapter' THEN 'chapter' ELSE 'block' END,
			b.id::text,
			b.title
		FROM blocks b
		WHERE b.id = $1 AND b.project_id = $2
	`, blockID, projectID).Scan(
		&source.TargetType,
		&source.TargetID,
		&title,
	)
	if err != nil {
		return BlockSummarySource{}, normalizeNotFound(err)
	}
	if title.Valid {
		source.Title = title.String
	}

	scopeClause := "b.id = $2"
	if source.TargetType == "chapter" {
		scopeClause = "(b.id = $2 OR b.parent_block_id = $2)"
	}
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT b.title, br.id::text, br.content, br.content_format
		FROM blocks b
		JOIN block_revisions br ON br.id = b.current_revision_id
		WHERE b.project_id = $1 AND %s
		ORDER BY b.order_index, b.created_at
	`, scopeClause), projectID, blockID)
	if err != nil {
		return BlockSummarySource{}, err
	}
	defer rows.Close()
	var sections []string
	for rows.Next() {
		var blockTitle sql.NullString
		var revisionID, content, contentFormat string
		if err := rows.Scan(&blockTitle, &revisionID, &content, &contentFormat); err != nil {
			return BlockSummarySource{}, err
		}
		source.CoveredRevisionIDs = append(source.CoveredRevisionIDs, revisionID)
		sections = append(sections, "## "+fallbackTitle(nullableText(blockTitle), "未命名片段")+"\n"+normalizeBlockContent(content, contentFormat))
	}
	if err := rows.Err(); err != nil {
		return BlockSummarySource{}, err
	}
	source.Content = strings.Join(sections, "\n\n")
	return source, nil
}

func (r *Repository) GetBranchSummarySource(ctx context.Context, projectID string, branchID string) (BlockSummarySource, error) {
	source := BlockSummarySource{TargetType: "branch", TargetID: branchID}
	if err := r.db.QueryRow(ctx, `
		SELECT name FROM branches WHERE id = $1 AND project_id = $2
	`, branchID, projectID).Scan(&source.Title); err != nil {
		return BlockSummarySource{}, normalizeNotFound(err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT b.title, br.id::text, br.content, br.content_format
		FROM blocks b
		JOIN block_revisions br ON br.id = b.current_revision_id
		WHERE b.project_id = $1 AND b.branch_id = $2
		ORDER BY b.order_index, b.created_at
	`, projectID, branchID)
	if err != nil {
		return BlockSummarySource{}, err
	}
	defer rows.Close()
	var sections []string
	for rows.Next() {
		var title sql.NullString
		var revisionID, content, contentFormat string
		if err := rows.Scan(&title, &revisionID, &content, &contentFormat); err != nil {
			return BlockSummarySource{}, err
		}
		source.CoveredRevisionIDs = append(source.CoveredRevisionIDs, revisionID)
		sections = append(sections, "## "+fallbackTitle(nullableText(title), "未命名片段")+"\n"+normalizeBlockContent(content, contentFormat))
	}
	if err := rows.Err(); err != nil {
		return BlockSummarySource{}, err
	}
	source.Content = strings.Join(sections, "\n\n")
	return source, nil
}

func (r *Repository) GetSummarySource(ctx context.Context, projectID string, summaryID string) (BlockSummarySource, error) {
	var targetType, targetID string
	if err := r.db.QueryRow(ctx, `
		SELECT target_type, target_id::text
		FROM summary_snapshots
		WHERE id = $1 AND project_id = $2
	`, summaryID, projectID).Scan(&targetType, &targetID); err != nil {
		return BlockSummarySource{}, normalizeNotFound(err)
	}
	if targetType == "branch" {
		return r.GetBranchSummarySource(ctx, projectID, targetID)
	}
	return r.GetBlockSummarySource(ctx, projectID, targetID)
}

func (r *Repository) CreateSummary(ctx context.Context, projectID string, source BlockSummarySource, result CompletionResult, model string) (SummarySnapshot, error) {
	tokenCount := result.OutputTokens
	if tokenCount <= 0 {
		tokenCount = estimateTokens(result.Content)
	}
	metadata, err := json.Marshal(map[string]any{
		"input_tokens":  result.InputTokens,
		"output_tokens": result.OutputTokens,
	})
	if err != nil {
		return SummarySnapshot{}, err
	}
	return r.createSummarySnapshot(ctx, projectID, source, result.Content, tokenCount, model, "valid", metadata)
}

func (r *Repository) CreateFailedSummary(ctx context.Context, projectID string, source BlockSummarySource, model string, failure error) (SummarySnapshot, error) {
	metadata, err := json.Marshal(map[string]any{"error": failure.Error()})
	if err != nil {
		return SummarySnapshot{}, err
	}
	return r.createSummarySnapshot(ctx, projectID, source, "", 0, model, "failed", metadata)
}

func (r *Repository) createSummarySnapshot(
	ctx context.Context,
	projectID string,
	source BlockSummarySource,
	summaryText string,
	tokenCount int,
	model string,
	status string,
	metadata json.RawMessage,
) (SummarySnapshot, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return SummarySnapshot{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE summary_snapshots
		SET status = 'stale'
		WHERE project_id = $1 AND target_type = $2 AND target_id = $3 AND status = 'valid'
	`, projectID, source.TargetType, source.TargetID); err != nil {
		return SummarySnapshot{}, err
	}

	var snapshot SummarySnapshot
	err = tx.QueryRow(ctx, `
		INSERT INTO summary_snapshots (
			project_id,
			target_type,
			target_id,
			summary_text,
			covered_revision_ids,
			token_count,
			model,
			status,
			metadata
		)
		VALUES ($1, $2, $3, $4, $5::uuid[], $6, $7, $8, $9)
		RETURNING
			id::text,
			project_id::text,
			target_type,
			target_id::text,
			summary_text,
			covered_revision_ids::text[],
			token_count,
			model,
			status,
			metadata,
			created_at
	`, projectID, source.TargetType, source.TargetID, summaryText, source.CoveredRevisionIDs, tokenCount, model, status, metadata).Scan(
		&snapshot.ID,
		&snapshot.ProjectID,
		&snapshot.TargetType,
		&snapshot.TargetID,
		&snapshot.SummaryText,
		&snapshot.CoveredRevisionIDs,
		&snapshot.TokenCount,
		&snapshot.Model,
		&snapshot.Status,
		&snapshot.Metadata,
		&snapshot.CreatedAt,
	)
	if err != nil {
		return SummarySnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SummarySnapshot{}, err
	}
	return snapshot, nil
}

func (r *Repository) ListSummaries(ctx context.Context, projectID string) ([]SummarySnapshot, error) {
	if err := r.RefreshStaleSummaryStatuses(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (target_type, target_id)
			id::text,
			project_id::text,
			target_type,
			target_id::text,
			summary_text,
			covered_revision_ids::text[],
			token_count,
			model,
			status,
			metadata,
			created_at
		FROM summary_snapshots
		WHERE project_id = $1
		ORDER BY target_type, target_id, created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	summaries := make([]SummarySnapshot, 0)
	for rows.Next() {
		var snapshot SummarySnapshot
		if err := rows.Scan(
			&snapshot.ID, &snapshot.ProjectID, &snapshot.TargetType, &snapshot.TargetID,
			&snapshot.SummaryText, &snapshot.CoveredRevisionIDs, &snapshot.TokenCount,
			&snapshot.Model, &snapshot.Status, &snapshot.Metadata, &snapshot.CreatedAt,
		); err != nil {
			return nil, err
		}
		summaries = append(summaries, snapshot)
	}
	return summaries, rows.Err()
}

func (r *Repository) RefreshStaleSummaryStatuses(ctx context.Context, projectID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE summary_snapshots summary
		SET status = 'stale'
		WHERE summary.project_id = $1
			AND summary.status = 'valid'
			AND (
				SELECT COALESCE(array_agg(revision_id ORDER BY revision_id), '{}'::uuid[])
				FROM unnest(summary.covered_revision_ids) AS revision_id
			) IS DISTINCT FROM (
				CASE summary.target_type
					WHEN 'block' THEN ARRAY(
						SELECT block.current_revision_id
						FROM blocks block
						WHERE block.project_id = summary.project_id
							AND block.id = summary.target_id
							AND block.current_revision_id IS NOT NULL
						ORDER BY block.current_revision_id
					)
					WHEN 'chapter' THEN ARRAY(
						SELECT block.current_revision_id
						FROM blocks block
						WHERE block.project_id = summary.project_id
							AND (block.id = summary.target_id OR block.parent_block_id = summary.target_id)
							AND block.current_revision_id IS NOT NULL
						ORDER BY block.current_revision_id
					)
					WHEN 'branch' THEN ARRAY(
						SELECT block.current_revision_id
						FROM blocks block
						WHERE block.project_id = summary.project_id
							AND block.branch_id = summary.target_id
							AND block.current_revision_id IS NOT NULL
						ORDER BY block.current_revision_id
					)
					ELSE '{}'::uuid[]
				END
			)
	`, projectID)
	return err
}

func nullableText(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func (r *Repository) GetPromptTemplate(ctx context.Context, projectID string, templateID string) (PromptTemplate, error) {
	var template PromptTemplate
	err := r.db.QueryRow(ctx, `
		SELECT id::text, task_type, template_text
		FROM prompt_templates
		WHERE id = $1 AND project_id = $2
	`, templateID, projectID).Scan(&template.ID, &template.TaskType, &template.TemplateText)
	if err != nil {
		return PromptTemplate{}, normalizeNotFound(err)
	}
	return template, nil
}

func (r *Repository) GetDefaultPromptTemplate(ctx context.Context, projectID string, taskType string) (PromptTemplate, error) {
	var template PromptTemplate
	err := r.db.QueryRow(ctx, `
		SELECT id::text, task_type, template_text
		FROM prompt_templates
		WHERE project_id = $1 AND task_type = $2 AND is_default = true
		ORDER BY updated_at DESC
		LIMIT 1
	`, projectID, taskType).Scan(&template.ID, &template.TaskType, &template.TemplateText)
	if err != nil {
		return PromptTemplate{}, normalizeNotFound(err)
	}
	return template, nil
}

func (r *Repository) CreateRun(ctx context.Context, input GenerationRunInput) (GenerationRun, error) {
	run, err := scanGenerationRun(r.db.QueryRow(ctx, `
		INSERT INTO generation_runs (
			project_id,
			block_id,
			task_type,
			provider,
			model,
			temperature,
			top_p,
			max_tokens,
			context_window,
			prompt_template_id,
			input_context_snapshot,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'running')
		RETURNING
			id::text,
			project_id::text,
			block_id::text,
			task_type,
			provider,
			model,
			temperature,
			top_p,
			max_tokens,
			context_window,
			prompt_template_id::text,
			input_context_snapshot,
			output_revision_id::text,
			input_tokens,
			output_tokens,
			latency_ms,
			status,
			error_message,
			created_at
	`, input.ProjectID, nullableString(input.BlockID), input.TaskType, input.Provider, input.Model, input.Temperature, input.TopP, input.MaxTokens, input.ContextWindow, nullableString(input.PromptTemplateID), input.InputContextSnapshot))
	if err != nil {
		return GenerationRun{}, err
	}
	return run, nil
}

func (r *Repository) MarkRunSucceeded(ctx context.Context, runID string, result CompletionResult, latencyMS int) (GenerationRun, error) {
	run, err := scanGenerationRun(r.db.QueryRow(ctx, updateGenerationRunSQL, "succeeded", nil, result.InputTokens, result.OutputTokens, latencyMS, runID))
	if err != nil {
		return GenerationRun{}, normalizeNotFound(err)
	}
	return run, nil
}

func (r *Repository) MarkRunFailed(ctx context.Context, runID string, message string, latencyMS int) (GenerationRun, error) {
	run, err := scanGenerationRun(r.db.QueryRow(ctx, updateGenerationRunSQL, "failed", message, 0, 0, latencyMS, runID))
	if err != nil {
		return GenerationRun{}, normalizeNotFound(err)
	}
	return run, nil
}

const updateGenerationRunSQL = `
	UPDATE generation_runs
	SET
		status = $1,
		error_message = $2,
		input_tokens = $3,
		output_tokens = $4,
		latency_ms = $5
	WHERE id = $6
	RETURNING
		id::text,
		project_id::text,
		block_id::text,
		task_type,
		provider,
		model,
		temperature,
		top_p,
		max_tokens,
		context_window,
		prompt_template_id::text,
		input_context_snapshot,
		output_revision_id::text,
		input_tokens,
		output_tokens,
		latency_ms,
		status,
		error_message,
		created_at
`

type scanner interface {
	Scan(dest ...any) error
}

func scanGenerationRun(scanner scanner) (GenerationRun, error) {
	var run GenerationRun
	var blockID sql.NullString
	var temperature sql.NullFloat64
	var topP sql.NullFloat64
	var maxTokens sql.NullInt64
	var contextWindow sql.NullInt64
	var promptTemplateID sql.NullString
	var outputRevisionID sql.NullString
	var errorMessage sql.NullString

	err := scanner.Scan(
		&run.ID,
		&run.ProjectID,
		&blockID,
		&run.TaskType,
		&run.Provider,
		&run.Model,
		&temperature,
		&topP,
		&maxTokens,
		&contextWindow,
		&promptTemplateID,
		&run.InputContextSnapshot,
		&outputRevisionID,
		&run.InputTokens,
		&run.OutputTokens,
		&run.LatencyMS,
		&run.Status,
		&errorMessage,
		&run.CreatedAt,
	)
	if err != nil {
		return GenerationRun{}, err
	}
	if blockID.Valid {
		run.BlockID = &blockID.String
	}
	if temperature.Valid {
		run.Temperature = &temperature.Float64
	}
	if topP.Valid {
		run.TopP = &topP.Float64
	}
	if maxTokens.Valid {
		value := int(maxTokens.Int64)
		run.MaxTokens = &value
	}
	if contextWindow.Valid {
		value := int(contextWindow.Int64)
		run.ContextWindow = &value
	}
	if promptTemplateID.Valid {
		run.PromptTemplateID = &promptTemplateID.String
	}
	if outputRevisionID.Valid {
		run.OutputRevisionID = &outputRevisionID.String
	}
	if errorMessage.Valid {
		run.ErrorMessage = &errorMessage.String
	}
	if len(run.InputContextSnapshot) == 0 {
		run.InputContextSnapshot = json.RawMessage(`{}`)
	}
	return run, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrGenerationResourceNotFound
	}
	return err
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func providerError(message string) error {
	return fmt.Errorf("%w: %s", ErrProviderRequestFailed, message)
}
