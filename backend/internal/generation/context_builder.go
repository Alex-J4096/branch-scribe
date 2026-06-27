package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	defaultContextBudgetTokens = 12000
	reservedOutputTokens       = 1200
)

var keywordPattern = regexp.MustCompile(`[\p{Han}]{2,}|[A-Za-z0-9]{3,}`)

type contextSnapshot struct {
	ProjectID       string         `json:"project_id"`
	BlockID         string         `json:"block_id"`
	TaskType        string         `json:"task_type"`
	SelectedText    string         `json:"selected_text"`
	UserInstruction string         `json:"user_instruction"`
	ContextPreview  ContextPreview `json:"context_preview"`
	Items           []ContextItem  `json:"items"`
}

func (h *Handler) BuildContextPreview(ctx context.Context, req GenerateOnceRequest) (ContextPreview, error) {
	req, err := req.normalized()
	if err != nil {
		return ContextPreview{}, err
	}

	modelProfile, err := h.repo.GetModelProfileForPreview(ctx, req.ProjectID, req.ModelProfileID)
	if err != nil {
		return ContextPreview{}, err
	}
	blockContext, template, err := h.loadPromptInputs(ctx, req)
	if err != nil {
		return ContextPreview{}, err
	}
	return h.buildContext(ctx, req, blockContext, template, contextBudget(modelProfile))
}

func (h *Handler) loadPromptInputs(ctx context.Context, req GenerateOnceRequest) (BlockContext, *PromptTemplate, error) {
	var blockContext BlockContext
	var err error
	if req.TaskType == "free_write" {
		blockContext, err = h.repo.GetBlockMetadataContext(ctx, req.ProjectID, req.BlockID)
	} else {
		blockContext, err = h.repo.GetBlockContext(ctx, req.ProjectID, req.BlockID)
	}
	if err != nil {
		return BlockContext{}, nil, err
	}

	var template *PromptTemplate
	if req.PromptTemplateID != nil {
		found, err := h.repo.GetPromptTemplate(ctx, req.ProjectID, *req.PromptTemplateID)
		if err != nil {
			return BlockContext{}, nil, err
		}
		template = &found
	} else if found, err := h.repo.GetDefaultPromptTemplate(ctx, req.ProjectID, req.TaskType); err == nil {
		template = &found
	} else if err != nil && !errors.Is(err, ErrGenerationResourceNotFound) {
		return BlockContext{}, nil, err
	}

	return blockContext, template, nil
}

func (h *Handler) buildContext(ctx context.Context, req GenerateOnceRequest, blockContext BlockContext, template *PromptTemplate, tokenBudget int) (ContextPreview, error) {
	currentBlock := normalizeBlockContent(blockContext.Content, blockContext.ContentFormat)
	keywords := extractContextKeywords(req, currentBlock, blockContext.CanonFacts)

	recentBlocks, err := h.repo.ListRecentBlocks(ctx, req.ProjectID, req.BlockID, 4)
	if err != nil {
		return ContextPreview{}, err
	}
	memories, err := h.repo.ListMemoryForContext(ctx, req.ProjectID, keywords, 5)
	if err != nil {
		return ContextPreview{}, err
	}
	summaries, err := h.repo.ListSummariesForContext(ctx, req.ProjectID, req.BlockID, blockContext.BranchID)
	if err != nil {
		return ContextPreview{}, err
	}

	items := make([]ContextItem, 0)
	items = append(items, ContextItem{
		ID:       "current_block",
		Type:     "current_block",
		Title:    fallbackTitle(blockContext.BlockTitle, "当前片段"),
		Content:  currentBlock,
		SourceID: req.BlockID,
		Required: req.TaskType != "free_write",
	})
	items = append(items, ContextItem{
		ID:       "canon_facts",
		Type:     "canon",
		Title:    "硬设定",
		Content:  renderCanonFacts(blockContext.CanonFacts),
		Required: true,
	})
	if req.SelectedText != "" {
		items = append(items, ContextItem{
			ID:       "selected_text",
			Type:     "selected_text",
			Title:    "选中文本",
			Content:  req.SelectedText,
			Required: req.TaskType == "rewrite_selection",
		})
	}
	for _, summary := range summaries {
		items = append(items, ContextItem{
			ID:       "summary:" + summary.ID,
			Type:     summary.TargetType + "_summary",
			Title:    summaryTitle(summary.TargetType),
			Content:  summary.SummaryText,
			SourceID: summary.ID,
		})
	}
	for _, block := range recentBlocks {
		items = append(items, ContextItem{
			ID:       "recent_block:" + block.ID,
			Type:     "recent_block",
			Title:    fallbackTitle(block.Title, fmt.Sprintf("前文 #%d", block.OrderIndex)),
			Content:  normalizeBlockContent(block.Content, block.ContentFormat),
			SourceID: block.ID,
		})
	}
	for _, memory := range memories {
		items = append(items, ContextItem{
			ID:       "memory:" + memory.ID,
			Type:     "memory_chunk",
			Title:    memoryTitle(memory),
			Content:  memory.ChunkText,
			SourceID: memory.ID,
		})
	}

	items = applyContextBudget(items, req.ExcludedContextItemIDs, tokenBudget)
	contextText := renderContextText(items)
	userPrompt, templateID := renderUserPrompt(req, blockContext, contextText, template)
	systemPrompt := defaultSystemPrompt()
	finalPrompt := "System:\n" + systemPrompt + "\n\nUser:\n" + userPrompt

	return ContextPreview{
		SystemPrompt:     systemPrompt,
		UserPrompt:       userPrompt,
		FinalPrompt:      finalPrompt,
		EstimatedTokens:  estimateTokens(finalPrompt),
		TokenBudget:      tokenBudget,
		Items:            items,
		ExcludedItemIDs:  req.ExcludedContextItemIDs,
		PromptTemplateID: templateID,
	}, nil
}

func contextBudget(profile ModelProfile) int {
	if profile.ContextWindow <= 0 {
		return defaultContextBudgetTokens
	}
	budget := profile.ContextWindow - profile.MaxTokens - reservedOutputTokens
	if budget < 2000 {
		return 2000
	}
	if budget > defaultContextBudgetTokens {
		return defaultContextBudgetTokens
	}
	return budget
}

func applyContextBudget(items []ContextItem, excludedIDs []string, tokenBudget int) []ContextItem {
	excluded := map[string]bool{}
	for _, id := range excludedIDs {
		excluded[id] = true
	}

	used := 0
	for index := range items {
		items[index].EstimatedTokens = estimateTokens(items[index].Content)
		if excluded[items[index].ID] && !items[index].Required {
			items[index].Included = false
			continue
		}
		if items[index].Required || used+items[index].EstimatedTokens <= tokenBudget {
			items[index].Included = true
			used += items[index].EstimatedTokens
			continue
		}
		items[index].Included = false
	}
	return items
}

func renderContextText(items []ContextItem) map[string]string {
	sections := map[string][]string{
		"current_block":   {},
		"canon_facts":     {},
		"recent_blocks":   {},
		"branch_summary":  {},
		"chapter_summary": {},
		"memory_chunks":   {},
	}
	for _, item := range items {
		if !item.Included {
			continue
		}
		switch item.Type {
		case "current_block":
			sections["current_block"] = append(sections["current_block"], item.Content)
		case "canon":
			sections["canon_facts"] = append(sections["canon_facts"], item.Content)
		case "recent_block":
			sections["recent_blocks"] = append(sections["recent_blocks"], "- "+item.Title+"\n"+item.Content)
		case "branch_summary":
			sections["branch_summary"] = append(sections["branch_summary"], item.Content)
		case "chapter_summary":
			sections["chapter_summary"] = append(sections["chapter_summary"], item.Content)
		case "memory_chunk":
			sections["memory_chunks"] = append(sections["memory_chunks"], "- "+item.Title+"\n"+item.Content)
		}
	}

	result := map[string]string{}
	for key, values := range sections {
		if len(values) == 0 || strings.TrimSpace(strings.Join(values, "")) == "" {
			result[key] = "无"
			continue
		}
		result[key] = strings.Join(values, "\n\n")
	}
	return result
}

func snapshotForPreview(req GenerateOnceRequest, preview ContextPreview) json.RawMessage {
	snapshot, _ := json.Marshal(contextSnapshot{
		ProjectID:       req.ProjectID,
		BlockID:         req.BlockID,
		TaskType:        req.TaskType,
		SelectedText:    req.SelectedText,
		UserInstruction: req.UserInstruction,
		ContextPreview:  preview,
		Items:           preview.Items,
	})
	return snapshot
}

func extractContextKeywords(req GenerateOnceRequest, currentBlock string, facts []CanonFact) []string {
	seen := map[string]bool{}
	keywords := make([]string, 0)
	add := func(value string) {
		for _, match := range keywordPattern.FindAllString(value, -1) {
			match = strings.TrimSpace(match)
			if match == "" || seen[match] {
				continue
			}
			seen[match] = true
			keywords = append(keywords, match)
			if len(keywords) >= 12 {
				return
			}
		}
	}
	add(req.UserInstruction)
	add(req.SelectedText)
	for _, fact := range facts {
		add(fact.Name)
		for _, alias := range fact.Aliases {
			add(alias)
		}
	}
	add(currentBlock)
	return keywords
}

func fallbackTitle(title *string, fallback string) string {
	if title != nil && strings.TrimSpace(*title) != "" {
		return strings.TrimSpace(*title)
	}
	return fallback
}

func summaryTitle(targetType string) string {
	switch targetType {
	case "branch":
		return "分支摘要"
	case "chapter":
		return "章节摘要"
	default:
		return "摘要"
	}
}

func memoryTitle(memory MemoryContext) string {
	if len(memory.Tags) > 0 {
		return memory.ChunkKind + " · " + strings.Join(memory.Tags, " / ")
	}
	return memory.ChunkKind
}

func estimateTokens(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	runes := utf8.RuneCountInString(value)
	tokens := runes / 2
	if tokens < 1 {
		return 1
	}
	return tokens
}
