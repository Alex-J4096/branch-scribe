package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"branchscribe/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo     *Repository
	provider Provider
}

func NewHandler(repo *Repository, provider Provider) *Handler {
	return &Handler{repo: repo, provider: provider}
}

func RegisterRoutes(router gin.IRouter, handler *Handler) {
	router.GET("/blocks/:blockId/llm-conversations", handler.ListConversations)
	router.POST("/blocks/:blockId/llm-conversations", handler.CreateConversation)
	router.GET("/llm-conversations/:conversationId/messages", handler.ListConversationMessages)
	router.PATCH("/llm-conversations/:conversationId", handler.UpdateConversation)
	router.DELETE("/llm-conversations/:conversationId", handler.DeleteConversation)
	router.DELETE("/llm-conversations/:conversationId/messages", handler.DeleteConversationMessages)
	router.PATCH("/llm-messages/:messageId", handler.UpdateConversationMessage)
	router.POST("/generate/context-preview", handler.ContextPreview)
	router.POST("/generate/once", handler.GenerateOnce)
	router.POST("/generate/candidates", handler.GenerateCandidates)
	router.POST("/generate/stream", handler.GenerateStream)
	router.POST("/projects/:projectId/characters/:characterId/extract-card", handler.ExtractCharacterCard)
	router.POST("/projects/:projectId/blocks/:blockId/check-consistency", handler.CheckConsistency)
	router.POST("/projects/:projectId/blocks/:blockId/extract-events", handler.ExtractTimelineEvents)
	router.POST("/blocks/:blockId/summarize", handler.GenerateBlockSummary)
	router.POST("/blocks/:blockId/summaries", handler.CreateManualBlockSummary)
	router.POST("/branches/:branchId/summarize", handler.GenerateBranchSummary)
	router.POST("/summaries/:summaryId/refresh", handler.RefreshSummary)
	router.PATCH("/summaries/:summaryId", handler.UpdateManualSummary)
	router.GET("/projects/:projectId/summaries", handler.ListSummaries)
}

func (h *Handler) CheckConsistency(c *gin.Context) {
	var req BlockAnalysisRequest
	if !bindBlockAnalysisRequest(c, &req) {
		return
	}
	projectID, blockID := c.Param("projectId"), c.Param("blockId")
	blockContext, profile, err := h.loadBlockAnalysisInputs(c.Request.Context(), projectID, blockID, req.ModelProfileID)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	canonJSON, _ := json.Marshal(blockContext.CanonFacts)
	prompt := fmt.Sprintf(`检查当前小说正文是否违反给定 Canon 硬设定。只报告正文中有明确证据的冲突，不要把信息缺失当作冲突。
返回严格 JSON，不要 Markdown 代码块：
{"consistent":true,"summary":"简短结论","conflicts":[{"canon_entity_id":"必须来自输入 Canon 的 id","canon_name":"设定名","severity":"warning 或 error","claim":"正文中的冲突表述","canon_fact":"被违反的设定","explanation":"冲突原因","suggestion":"修订建议"}]}
没有冲突时 conflicts 必须是空数组且 consistent 为 true。

Canon：
%s

当前正文：
%s`, canonJSON, blockContext.Content)
	raw, run, err := h.runBlockAnalysis(c.Request.Context(), projectID, blockID, "check_consistency", profile, prompt, map[string]any{
		"block_content": blockContext.Content,
		"canon_facts":   blockContext.CanonFacts,
	})
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	result, err := parseConsistencyCheck(raw, blockContext.CanonFacts)
	if err != nil {
		_, _ = h.repo.MarkRunFailed(c.Request.Context(), run.ID, err.Error(), 0)
		api.RespondError(c, http.StatusBadGateway, "INVALID_CONSISTENCY_RESPONSE", "model returned an invalid consistency check")
		return
	}
	result.BlockID, result.Model, result.GenerationRunID = blockID, profile.Model, run.ID
	api.RespondOK(c, result)
}

func (h *Handler) ExtractTimelineEvents(c *gin.Context) {
	var req BlockAnalysisRequest
	if !bindBlockAnalysisRequest(c, &req) {
		return
	}
	projectID, blockID := c.Param("projectId"), c.Param("blockId")
	blockContext, profile, err := h.loadBlockAnalysisInputs(c.Request.Context(), projectID, blockID, req.ModelProfileID)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	canonJSON, _ := json.Marshal(blockContext.CanonFacts)
	prompt := fmt.Sprintf(`从当前小说正文提取明确发生的时间线事件。不要提取计划、假设或仅被提及的往事。
返回严格 JSON，不要 Markdown 代码块：
{"events":[{"title":"事件标题","description":"发生了什么","event_time":"正文中的时间表达或 null","sort_order":0,"canon_entity_id":"相关输入 Canon id 或 null"}]}
事件按正文发生顺序排列，sort_order 从 0 开始；没有事件时返回空数组。

可关联 Canon：
%s

当前正文：
%s`, canonJSON, blockContext.Content)
	raw, run, err := h.runBlockAnalysis(c.Request.Context(), projectID, blockID, "extract_timeline_events", profile, prompt, map[string]any{
		"block_content": blockContext.Content,
		"canon_facts":   blockContext.CanonFacts,
	})
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	result, err := parseTimelineExtraction(raw, blockContext.CanonFacts)
	if err != nil {
		_, _ = h.repo.MarkRunFailed(c.Request.Context(), run.ID, err.Error(), 0)
		api.RespondError(c, http.StatusBadGateway, "INVALID_TIMELINE_EXTRACTION_RESPONSE", "model returned invalid timeline events")
		return
	}
	result.BlockID, result.Model, result.GenerationRunID = blockID, profile.Model, run.ID
	api.RespondOK(c, result)
}

func bindBlockAnalysisRequest(c *gin.Context, req *BlockAnalysisRequest) bool {
	if err := c.ShouldBindJSON(req); err != nil || strings.TrimSpace(req.ModelProfileID) == "" {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BLOCK_ANALYSIS_REQUEST", "model_profile_id is required")
		return false
	}
	req.ModelProfileID = strings.TrimSpace(req.ModelProfileID)
	return true
}

func (h *Handler) loadBlockAnalysisInputs(ctx context.Context, projectID, blockID, profileID string) (BlockContext, ModelProfile, error) {
	blockContext, err := h.repo.GetBlockContext(ctx, projectID, blockID)
	if err != nil {
		return BlockContext{}, ModelProfile{}, err
	}
	profile, err := h.repo.GetModelProfile(ctx, projectID, profileID)
	return blockContext, profile, err
}

func (h *Handler) runBlockAnalysis(ctx context.Context, projectID, blockID, taskType string, profile ModelProfile, prompt string, snapshotData map[string]any) (string, GenerationRun, error) {
	snapshot, _ := json.Marshal(snapshotData)
	run, err := h.repo.CreateRun(ctx, GenerationRunInput{
		ProjectID: projectID, BlockID: &blockID, TaskType: taskType, Provider: profile.Provider,
		Model: profile.Model, Temperature: profile.Temperature, TopP: profile.TopP,
		MaxTokens: profile.MaxTokens, ContextWindow: profile.ContextWindow, InputContextSnapshot: snapshot,
	})
	if err != nil {
		return "", GenerationRun{}, err
	}
	startedAt := time.Now()
	result, err := h.provider.GenerateOnce(ctx, GenerateRequest{
		Provider: profile.Provider, Model: profile.Model, BaseURL: stringValue(profile.BaseURL),
		APIKey: stringValue(profile.APIKey), Messages: []ChatMessage{
			{Role: "system", Content: "你是严谨的小说工程分析器，只依据提供的正文与设定输出结构化结果。"},
			{Role: "user", Content: prompt},
		},
		Temperature: profile.Temperature, TopP: profile.TopP, MaxTokens: profile.MaxTokens,
	})
	if err != nil {
		_, _ = h.repo.MarkRunFailed(ctx, run.ID, err.Error(), int(time.Since(startedAt).Milliseconds()))
		return "", run, err
	}
	if _, err := h.repo.MarkRunSucceeded(ctx, run.ID, result, int(time.Since(startedAt).Milliseconds())); err != nil {
		return "", run, err
	}
	return result.Content, run, nil
}

func parseConsistencyCheck(content string, facts []CanonFact) (ConsistencyCheckResult, error) {
	var result ConsistencyCheckResult
	if err := decodeStructuredJSON(content, &result); err != nil {
		return result, err
	}
	validIDs := make(map[string]bool, len(facts))
	for _, fact := range facts {
		validIDs[fact.ID] = true
	}
	for _, conflict := range result.Conflicts {
		if !validIDs[conflict.CanonEntityID] || strings.TrimSpace(conflict.Explanation) == "" ||
			(conflict.Severity != "warning" && conflict.Severity != "error") {
			return ConsistencyCheckResult{}, ErrInvalidGenerationRequest
		}
	}
	if len(result.Conflicts) > 0 {
		result.Consistent = false
	}
	if result.Conflicts == nil {
		result.Conflicts = []ConsistencyConflict{}
	}
	return result, nil
}

func parseTimelineExtraction(content string, facts []CanonFact) (TimelineExtractionResult, error) {
	var result TimelineExtractionResult
	if err := decodeStructuredJSON(content, &result); err != nil {
		return result, err
	}
	validIDs := make(map[string]bool, len(facts))
	for _, fact := range facts {
		validIDs[fact.ID] = true
	}
	for index := range result.Events {
		event := &result.Events[index]
		event.Title, event.Description = strings.TrimSpace(event.Title), strings.TrimSpace(event.Description)
		if event.Title == "" || event.Description == "" {
			return TimelineExtractionResult{}, ErrInvalidGenerationRequest
		}
		event.SortOrder = index
		if event.CanonEntityID != nil && !validIDs[*event.CanonEntityID] {
			event.CanonEntityID = nil
		}
	}
	if result.Events == nil {
		result.Events = []TimelineEventProposal{}
	}
	return result, nil
}

func decodeStructuredJSON(content string, target any) error {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return json.Unmarshal([]byte(strings.TrimSpace(content)), target)
}

func (h *Handler) ExtractCharacterCard(c *gin.Context) {
	var req ExtractCharacterCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CHARACTER_CARD_REQUEST", "invalid character card extraction request")
		return
	}
	req.BlockID = strings.TrimSpace(req.BlockID)
	req.ModelProfileID = strings.TrimSpace(req.ModelProfileID)
	if req.BlockID == "" || req.ModelProfileID == "" {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CHARACTER_CARD_REQUEST", "block_id and model_profile_id are required")
		return
	}
	blockIDs := uniqueNonEmptyStrings(req.BlockIDs)
	if len(blockIDs) == 0 {
		blockIDs = []string{req.BlockID}
	}
	if !slices.Contains(blockIDs, req.BlockID) || blockIDs[0] != req.BlockID {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CHARACTER_CARD_REQUEST", "block_ids must start with the starting block")
		return
	}

	projectID := c.Param("projectId")
	character, err := h.repo.GetCharacterCanonFact(c.Request.Context(), projectID, c.Param("characterId"))
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	sourceSections := make([]string, 0, len(blockIDs))
	var sourceBranchID *string
	startOrder := 0
	previousOrder := 0
	for index, blockID := range blockIDs {
		source, err := h.repo.GetBlockContext(c.Request.Context(), projectID, blockID)
		if err != nil {
			respondGenerationError(c, err)
			return
		}
		if index == 0 {
			sourceBranchID = source.BranchID
			startOrder = source.OrderIndex
			previousOrder = source.OrderIndex
		} else if !equalOptionalStrings(sourceBranchID, source.BranchID) ||
			source.OrderIndex < startOrder || source.OrderIndex < previousOrder {
			api.RespondError(c, http.StatusBadRequest, "INVALID_CHARACTER_CARD_REQUEST", "all blocks must be ordered descendants on the starting block branch")
			return
		}
		previousOrder = source.OrderIndex
		title := fmt.Sprintf("Block %d", index+1)
		if source.BlockTitle != nil {
			title = *source.BlockTitle
		}
		sourceSections = append(sourceSections, fmt.Sprintf("## %s\n%s", title, source.Content))
	}
	sourceContent := strings.Join(sourceSections, "\n\n")
	profile, err := h.repo.GetModelProfile(c.Request.Context(), projectID, req.ModelProfileID)
	if err != nil {
		respondGenerationError(c, err)
		return
	}

	characterJSON, _ := json.Marshal(character)
	prompt := fmt.Sprintf(`请根据后续剧情更新角色卡。只总结剧情中明确出现或可可靠推断的新状态，不要虚构。
返回严格 JSON，不要 Markdown 代码块，格式为：
{"description":"更新后的完整角色描述","attributes":{},"change_summary":"相对旧角色卡的变化摘要"}

旧角色卡：
%s

后续剧情：
%s`, characterJSON, sourceContent)
	snapshot, _ := json.Marshal(map[string]any{
		"character_id":     character.ID,
		"source_block_id":  req.BlockID,
		"source_block_ids": blockIDs,
		"character_card":   character,
		"source_content":   sourceContent,
	})
	blockID := req.BlockID
	run, err := h.repo.CreateRun(c.Request.Context(), GenerationRunInput{
		ProjectID:            projectID,
		BlockID:              &blockID,
		TaskType:             "extract_character_card",
		Provider:             profile.Provider,
		Model:                profile.Model,
		Temperature:          profile.Temperature,
		TopP:                 profile.TopP,
		MaxTokens:            profile.MaxTokens,
		ContextWindow:        profile.ContextWindow,
		InputContextSnapshot: snapshot,
	})
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	startedAt := time.Now()
	result, err := h.provider.GenerateOnce(c.Request.Context(), GenerateRequest{
		Provider:    profile.Provider,
		Model:       profile.Model,
		BaseURL:     stringValue(profile.BaseURL),
		APIKey:      stringValue(profile.APIKey),
		Messages:    []ChatMessage{{Role: "system", Content: "你是小说角色设定编辑器，负责从后续剧情维护可追溯的角色卡版本。"}, {Role: "user", Content: prompt}},
		Temperature: profile.Temperature,
		TopP:        profile.TopP,
		MaxTokens:   profile.MaxTokens,
	})
	if err != nil {
		_, _ = h.repo.MarkRunFailed(c.Request.Context(), run.ID, err.Error(), int(time.Since(startedAt).Milliseconds()))
		respondGenerationError(c, err)
		return
	}
	proposal, err := parseCharacterCardProposal(result.Content)
	if err != nil {
		_, _ = h.repo.MarkRunFailed(c.Request.Context(), run.ID, err.Error(), int(time.Since(startedAt).Milliseconds()))
		api.RespondError(c, http.StatusBadGateway, "INVALID_CHARACTER_CARD_RESPONSE", "model returned an invalid character card")
		return
	}
	if _, err := h.repo.MarkRunSucceeded(c.Request.Context(), run.ID, result, int(time.Since(startedAt).Milliseconds())); err != nil {
		respondGenerationError(c, err)
		return
	}
	proposal.CharacterID = character.ID
	proposal.SourceBlockID = req.BlockID
	proposal.SourceBlockIDs = blockIDs
	proposal.Model = profile.Model
	proposal.GenerationRunID = run.ID
	api.RespondOK(c, proposal)
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func equalOptionalStrings(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func parseCharacterCardProposal(content string) (CharacterCardProposal, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	var proposal CharacterCardProposal
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &proposal); err != nil {
		return CharacterCardProposal{}, err
	}
	if strings.TrimSpace(proposal.Description) == "" || len(proposal.Attributes) == 0 || !json.Valid(proposal.Attributes) {
		return CharacterCardProposal{}, ErrInvalidGenerationRequest
	}
	return proposal, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (h *Handler) ListConversations(c *gin.Context) {
	items, err := h.repo.ListConversations(c.Request.Context(), c.Param("blockId"))
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	api.RespondOK(c, items)
}

func (h *Handler) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ProjectID) == "" {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CONVERSATION_REQUEST", "invalid conversation request")
		return
	}
	item, err := h.repo.CreateConversation(c.Request.Context(), c.Param("blockId"), req)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: item, Error: nil})
}

func (h *Handler) ListConversationMessages(c *gin.Context) {
	items, err := h.repo.ListConversationMessages(c.Request.Context(), c.Param("conversationId"))
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	api.RespondOK(c, items)
}

func (h *Handler) UpdateConversation(c *gin.Context) {
	var req UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CONVERSATION_REQUEST", "invalid conversation request")
		return
	}
	item, err := h.repo.UpdateConversation(c.Request.Context(), c.Param("conversationId"), req.Title)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	api.RespondOK(c, item)
}

func (h *Handler) DeleteConversation(c *gin.Context) {
	if err := h.repo.DeleteConversation(c.Request.Context(), c.Param("conversationId")); err != nil {
		respondGenerationError(c, err)
		return
	}
	api.RespondOK(c, gin.H{"deleted": true})
}

func (h *Handler) UpdateConversationMessage(c *gin.Context) {
	var req UpdateConversationMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CONVERSATION_REQUEST", "invalid message request")
		return
	}
	item, err := h.repo.UpdateConversationMessage(c.Request.Context(), c.Param("messageId"), req.Content)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	api.RespondOK(c, item)
}

func (h *Handler) DeleteConversationMessages(c *gin.Context) {
	var req DeleteConversationMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CONVERSATION_REQUEST", "invalid message deletion request")
		return
	}
	messageIDs, err := normalizeMessageIDs(req.MessageIDs)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	if err := h.repo.DeleteConversationMessages(c.Request.Context(), c.Param("conversationId"), messageIDs); err != nil {
		respondGenerationError(c, err)
		return
	}
	api.RespondOK(c, gin.H{"deleted": len(messageIDs)})
}

func (h *Handler) GenerateCandidates(c *gin.Context) {
	var req GenerateCandidatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_GENERATION_REQUEST", "invalid generation request")
		return
	}
	if req.Count == 0 {
		req.Count = 2
	}
	if req.Count != 2 {
		api.RespondError(c, http.StatusBadRequest, "INVALID_GENERATION_REQUEST", "candidate count must be 2")
		return
	}
	responses := make([]GenerateOnceResponse, 0, 2)
	for index := 0; index < 2; index++ {
		candidateRequest := req.GenerateOnceRequest
		candidateRequest.SkipConversationSave = true
		candidateRequest.UserInstruction = strings.TrimSpace(candidateRequest.UserInstruction) +
			fmt.Sprintf("\n生成候选版本 %d。它应与另一个候选在情节选择、表达或节奏上有实质区别，只输出候选正文。", index+1)
		response, err := h.generateOnce(c.Request.Context(), candidateRequest)
		if err != nil {
			respondGenerationError(c, err)
			return
		}
		responses = append(responses, response)
	}
	if req.ConversationID != nil {
		if _, err := h.repo.AppendConversationMessage(
			c.Request.Context(),
			*req.ConversationID,
			"user",
			conversationUserContent(req.GenerateOnceRequest),
			&responses[0].GenerationRun.ID,
		); err != nil {
			respondGenerationError(c, err)
			return
		}
		for _, response := range responses {
			if strings.TrimSpace(response.OutputText) == "" {
				continue
			}
			if _, err := h.repo.AppendConversationMessage(
				c.Request.Context(),
				*req.ConversationID,
				"assistant",
				response.OutputText,
				&response.GenerationRun.ID,
			); err != nil {
				respondGenerationError(c, err)
				return
			}
		}
	}
	api.RespondOK(c, GenerateCandidatesResponse{Candidates: responses})
}

func (h *Handler) ContextPreview(c *gin.Context) {
	var req GenerateOnceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_GENERATION_REQUEST", "invalid generation request")
		return
	}

	preview, err := h.BuildContextPreview(c.Request.Context(), req)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	api.RespondOK(c, preview)
}

func (h *Handler) GenerateOnce(c *gin.Context) {
	var req GenerateOnceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_GENERATION_REQUEST", "invalid generation request")
		return
	}

	response, err := h.generateOnce(c.Request.Context(), req)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	api.RespondOK(c, response)
}

func (h *Handler) GenerateBlockSummary(c *gin.Context) {
	var req GenerateBlockSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_SUMMARY_REQUEST", "invalid summary request")
		return
	}
	req, err := req.normalized()
	if err != nil {
		respondGenerationError(c, err)
		return
	}

	source, err := h.repo.GetBlockSummarySource(c.Request.Context(), req.ProjectID, c.Param("blockId"))
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	h.generateSummary(c, req, source)
}

func (h *Handler) CreateManualBlockSummary(c *gin.Context) {
	var req ManualSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_SUMMARY_REQUEST", "invalid summary request")
		return
	}
	req, err := req.normalized()
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	source, err := h.repo.GetBlockSummarySource(c.Request.Context(), req.ProjectID, c.Param("blockId"))
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	snapshot, err := h.repo.CreateManualSummary(c.Request.Context(), req.ProjectID, source, req.SummaryText)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "SUMMARY_SAVE_FAILED", "failed to save summary")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: snapshot, Error: nil})
}

func (h *Handler) UpdateManualSummary(c *gin.Context) {
	var req ManualSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_SUMMARY_REQUEST", "invalid summary request")
		return
	}
	req, err := req.normalized()
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	source, err := h.repo.GetSummarySource(c.Request.Context(), req.ProjectID, c.Param("summaryId"), "full_text", nil)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	if source.TargetType == "branch" {
		api.RespondError(c, http.StatusBadRequest, "INVALID_SUMMARY_REQUEST", "branch summaries cannot be edited here")
		return
	}
	snapshot, err := h.repo.CreateManualSummary(c.Request.Context(), req.ProjectID, source, req.SummaryText)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "SUMMARY_SAVE_FAILED", "failed to save summary")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: snapshot, Error: nil})
}

func (h *Handler) GenerateBranchSummary(c *gin.Context) {
	var req GenerateBranchSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_SUMMARY_REQUEST", "invalid summary request")
		return
	}
	req, err := req.normalized()
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	source, err := h.repo.GetBranchSummarySource(c.Request.Context(), req.ProjectID, c.Param("branchId"), req.SourceMode, req.SourceSelections)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	h.generateSummary(c, req, source)
}

func (h *Handler) RefreshSummary(c *gin.Context) {
	var req GenerateBlockSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_SUMMARY_REQUEST", "invalid summary request")
		return
	}
	req, err := req.normalized()
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	source, err := h.repo.GetSummarySource(c.Request.Context(), req.ProjectID, c.Param("summaryId"), req.SourceMode, req.SourceSelections)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	h.generateSummary(c, req, source)
}

func (h *Handler) ListSummaries(c *gin.Context) {
	summaries, err := h.repo.ListSummaries(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "SUMMARY_LIST_FAILED", "failed to list summaries")
		return
	}
	api.RespondOK(c, summaries)
}

func (h *Handler) generateSummary(c *gin.Context, req GenerateBlockSummaryRequest, source BlockSummarySource) {
	source.PromptTemplateID = req.PromptTemplateID
	profile, err := h.repo.GetModelProfile(c.Request.Context(), req.ProjectID, req.ModelProfileID)
	if err != nil {
		respondGenerationError(c, err)
		return
	}
	if !isOpenAICompatibleProvider(profile.Provider) {
		respondGenerationError(c, ErrUnsupportedProvider)
		return
	}
	if profile.APIKey == nil || *profile.APIKey == "" {
		respondGenerationError(c, fmt.Errorf("%w: model profile has no api key", ErrInvalidGenerationRequest))
		return
	}

	content := strings.TrimSpace(source.Content)
	if content == "" {
		api.RespondError(c, http.StatusBadRequest, "INVALID_SUMMARY_REQUEST", "summary source content is empty")
		return
	}
	title := strings.TrimSpace(source.Title)
	if title == "" {
		title = "未命名内容"
	}
	baseURL := ""
	if profile.BaseURL != nil {
		baseURL = *profile.BaseURL
	}
	providerCtx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	taskType := source.TargetType + "_summary"
	templateText := "请准确、简洁地概括以下小说内容，保留关键人物、事件、因果、地点与未解决冲突，不添加原文没有的信息。只输出摘要正文。\n\n内容类型：{{target_type}}\n标题：{{title}}\n\n正文：\n{{content}}"
	if req.PromptTemplateID != nil {
		template, templateErr := h.repo.GetPromptTemplate(c.Request.Context(), req.ProjectID, *req.PromptTemplateID)
		if templateErr != nil {
			respondGenerationError(c, templateErr)
			return
		}
		if template.TaskType != taskType {
			respondGenerationError(c, ErrInvalidGenerationRequest)
			return
		}
		templateText = template.TemplateText
	} else if template, templateErr := h.repo.GetDefaultPromptTemplate(c.Request.Context(), req.ProjectID, taskType); templateErr == nil {
		templateText = template.TemplateText
	}
	prompt := strings.NewReplacer(
		"{{target_type}}", source.TargetType,
		"{{title}}", title,
		"{{content}}", content,
	).Replace(templateText)
	result, err := h.provider.GenerateOnce(providerCtx, GenerateRequest{
		Provider: profile.Provider,
		Model:    profile.Model,
		BaseURL:  baseURL,
		APIKey:   *profile.APIKey,
		Messages: []ChatMessage{
			{Role: "system", Content: "你是小说编辑，请严格执行用户提供的摘要任务。"},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		TopP:        profile.TopP,
		MaxTokens:   min(profile.MaxTokens, 800),
	})
	if err != nil {
		_, _ = h.repo.CreateFailedSummary(c.Request.Context(), req.ProjectID, source, profile.Model, err)
		respondGenerationError(c, err)
		return
	}
	result.Content = strings.TrimSpace(result.Content)
	if result.Content == "" {
		emptySummaryErr := errors.New("provider returned an empty summary")
		_, _ = h.repo.CreateFailedSummary(c.Request.Context(), req.ProjectID, source, profile.Model, emptySummaryErr)
		api.RespondError(c, http.StatusBadGateway, "SUMMARY_GENERATION_FAILED", "provider returned an empty summary")
		return
	}
	snapshot, err := h.repo.CreateSummary(c.Request.Context(), req.ProjectID, source, result, profile.Model)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "SUMMARY_SAVE_FAILED", "failed to save summary")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: snapshot, Error: nil})
}

func (h *Handler) GenerateStream(c *gin.Context) {
	var req GenerateOnceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_GENERATION_REQUEST", "invalid generation request")
		return
	}

	prepared, err := h.prepareGeneration(c.Request.Context(), req)
	if err != nil {
		respondGenerationError(c, err)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	startedAt := time.Now()
	stream, err := h.provider.GenerateStream(c.Request.Context(), GenerateRequest{
		Provider:    prepared.ModelProfile.Provider,
		Model:       prepared.ModelProfile.Model,
		BaseURL:     prepared.BaseURL,
		APIKey:      *prepared.ModelProfile.APIKey,
		Messages:    prepared.Messages,
		Temperature: prepared.ModelProfile.Temperature,
		TopP:        prepared.ModelProfile.TopP,
		MaxTokens:   prepared.ModelProfile.MaxTokens,
		Stream:      true,
	})
	if err != nil {
		latencyMS := int(time.Since(startedAt).Milliseconds())
		run := prepared.Run
		if failedRun, updateErr := h.repo.MarkRunFailed(c.Request.Context(), prepared.Run.ID, err.Error(), latencyMS); updateErr == nil {
			run = failedRun
		}
		writeSSE(c, GenerateStreamEvent{
			Type:             "error",
			Error:            err.Error(),
			GenerationRun:    &run,
			Prompt:           prepared.Prompt,
			SystemPrompt:     prepared.ContextPreview.SystemPrompt,
			UserPrompt:       prepared.ContextPreview.UserPrompt,
			ContextPreview:   &prepared.ContextPreview,
			ModelProfileID:   prepared.Request.ModelProfileID,
			PromptTemplateID: prepared.PromptTemplateID,
			ConversationID:   prepared.Request.ConversationID,
		})
		return
	}

	var output string
	var reasoning string
	firstTokenLatencyMS := 0
	for event := range stream {
		switch event.Type {
		case "delta":
			if firstTokenLatencyMS == 0 {
				firstTokenLatencyMS = int(time.Since(startedAt).Milliseconds())
			}
			output += event.Content
			writeSSE(c, GenerateStreamEvent{Type: "delta", Content: event.Content})
		case "reasoning":
			if firstTokenLatencyMS == 0 {
				firstTokenLatencyMS = int(time.Since(startedAt).Milliseconds())
			}
			reasoning += event.Reasoning
			writeSSE(c, GenerateStreamEvent{Type: "reasoning", Reasoning: event.Reasoning})
		case "error":
			latencyMS := int(time.Since(startedAt).Milliseconds())
			run := prepared.Run
			message := event.Error
			if message == "" {
				message = "provider stream failed"
			}
			if failedRun, updateErr := h.repo.MarkRunFailed(c.Request.Context(), prepared.Run.ID, message, latencyMS); updateErr == nil {
				run = failedRun
			}
			writeSSE(c, GenerateStreamEvent{
				Type:             "error",
				Error:            message,
				GenerationRun:    &run,
				Prompt:           prepared.Prompt,
				SystemPrompt:     prepared.ContextPreview.SystemPrompt,
				UserPrompt:       prepared.ContextPreview.UserPrompt,
				ContextPreview:   &prepared.ContextPreview,
				ModelProfileID:   prepared.Request.ModelProfileID,
				PromptTemplateID: prepared.PromptTemplateID,
				ConversationID:   prepared.Request.ConversationID,
			})
			return
		case "done":
			latencyMS := int(time.Since(startedAt).Milliseconds())
			if firstTokenLatencyMS == 0 {
				firstTokenLatencyMS = latencyMS
			}
			run := prepared.Run
			succeededRun, updateErr := h.repo.MarkRunSucceeded(c.Request.Context(), prepared.Run.ID, CompletionResult{
				Content:      output,
				InputTokens:  event.InputTokens,
				OutputTokens: event.OutputTokens,
				FinishReason: event.FinishReason,
			}, latencyMS, firstTokenLatencyMS)
			if updateErr != nil {
				writeSSE(c, GenerateStreamEvent{Type: "error", Error: "failed to update generation run"})
				return
			}
			run = succeededRun
			if prepared.Request.ConversationID != nil && strings.TrimSpace(output) != "" {
				var saveErr error
				if prepared.Request.RegenerateMessageID != nil {
					saveErr = h.repo.ReplaceConversationAssistant(
						c.Request.Context(),
						*prepared.Request.ConversationID,
						*prepared.Request.RegenerateMessageID,
						output,
						run.ID,
					)
				} else {
					_, saveErr = h.repo.AppendConversationMessage(c.Request.Context(), *prepared.Request.ConversationID, "assistant", output, &run.ID)
				}
				if saveErr != nil {
					writeSSE(c, GenerateStreamEvent{Type: "error", Error: "failed to save conversation reply"})
					return
				}
			}
			writeSSE(c, GenerateStreamEvent{
				Type:             "done",
				Reasoning:        reasoning,
				GenerationRun:    &run,
				Prompt:           prepared.Prompt,
				SystemPrompt:     prepared.ContextPreview.SystemPrompt,
				UserPrompt:       prepared.ContextPreview.UserPrompt,
				ContextPreview:   &prepared.ContextPreview,
				ModelProfileID:   prepared.Request.ModelProfileID,
				PromptTemplateID: prepared.PromptTemplateID,
				ConversationID:   prepared.Request.ConversationID,
			})
			return
		}
	}
}

func (h *Handler) generateOnce(ctx context.Context, req GenerateOnceRequest) (GenerateOnceResponse, error) {
	prepared, err := h.prepareGeneration(ctx, req)
	if err != nil {
		return GenerateOnceResponse{}, err
	}

	providerCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	startedAt := time.Now()
	result, err := h.provider.GenerateOnce(providerCtx, GenerateRequest{
		Provider:    prepared.ModelProfile.Provider,
		Model:       prepared.ModelProfile.Model,
		BaseURL:     prepared.BaseURL,
		APIKey:      *prepared.ModelProfile.APIKey,
		Messages:    prepared.Messages,
		Temperature: prepared.ModelProfile.Temperature,
		TopP:        prepared.ModelProfile.TopP,
		MaxTokens:   prepared.ModelProfile.MaxTokens,
		Stream:      false,
	})
	latencyMS := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		failedRun, updateErr := h.repo.MarkRunFailed(ctx, prepared.Run.ID, err.Error(), latencyMS)
		run := prepared.Run
		if updateErr == nil {
			run = failedRun
		}
		return GenerateOnceResponse{
			GenerationRun:    run,
			Prompt:           prepared.Prompt,
			SystemPrompt:     prepared.ContextPreview.SystemPrompt,
			UserPrompt:       prepared.ContextPreview.UserPrompt,
			ContextPreview:   prepared.ContextPreview,
			ModelProfileID:   prepared.Request.ModelProfileID,
			PromptTemplateID: prepared.PromptTemplateID,
			ConversationID:   prepared.Request.ConversationID,
		}, err
	}

	run, err := h.repo.MarkRunSucceeded(ctx, prepared.Run.ID, result, latencyMS)
	if err != nil {
		return GenerateOnceResponse{}, err
	}
	if prepared.Request.ConversationID != nil && !prepared.Request.SkipConversationSave && strings.TrimSpace(result.Content) != "" {
		var saveErr error
		if prepared.Request.RegenerateMessageID != nil {
			saveErr = h.repo.ReplaceConversationAssistant(
				ctx,
				*prepared.Request.ConversationID,
				*prepared.Request.RegenerateMessageID,
				result.Content,
				run.ID,
			)
		} else {
			_, saveErr = h.repo.AppendConversationMessage(ctx, *prepared.Request.ConversationID, "assistant", result.Content, &run.ID)
		}
		if saveErr != nil {
			return GenerateOnceResponse{}, saveErr
		}
	}
	return GenerateOnceResponse{
		OutputText:       result.Content,
		ReasoningText:    result.Reasoning,
		GenerationRun:    run,
		Prompt:           prepared.Prompt,
		SystemPrompt:     prepared.ContextPreview.SystemPrompt,
		UserPrompt:       prepared.ContextPreview.UserPrompt,
		ContextPreview:   prepared.ContextPreview,
		ModelProfileID:   prepared.Request.ModelProfileID,
		PromptTemplateID: prepared.PromptTemplateID,
		ConversationID:   prepared.Request.ConversationID,
	}, nil
}

type preparedGeneration struct {
	Request          GenerateOnceRequest
	ModelProfile     ModelProfile
	Run              GenerationRun
	Prompt           string
	Messages         []ChatMessage
	ContextPreview   ContextPreview
	PromptTemplateID *string
	BaseURL          string
}

func (h *Handler) prepareGeneration(ctx context.Context, req GenerateOnceRequest) (preparedGeneration, error) {
	req, err := req.normalized()
	if err != nil {
		return preparedGeneration{}, err
	}

	modelProfile, err := h.repo.GetModelProfile(ctx, req.ProjectID, req.ModelProfileID)
	if err != nil {
		return preparedGeneration{}, err
	}
	if !isOpenAICompatibleProvider(modelProfile.Provider) {
		return preparedGeneration{}, ErrUnsupportedProvider
	}
	if modelProfile.APIKey == nil || *modelProfile.APIKey == "" {
		return preparedGeneration{}, fmt.Errorf("%w: model profile has no api key", ErrInvalidGenerationRequest)
	}
	if req.Temperature != nil {
		modelProfile.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		modelProfile.TopP = *req.TopP
	}
	if req.MaxTokens != nil {
		modelProfile.MaxTokens = *req.MaxTokens
	}

	blockContext, template, err := h.loadPromptInputs(ctx, req)
	if err != nil {
		return preparedGeneration{}, err
	}

	contextPreview, err := h.buildContext(ctx, req, blockContext, template, contextBudget(modelProfile))
	if err != nil {
		return preparedGeneration{}, err
	}

	prompt := contextPreview.FinalPrompt
	snapshot := snapshotForPreview(req, contextPreview)
	blockID := req.BlockID
	promptTemplateID := contextPreview.PromptTemplateID

	run, err := h.repo.CreateRun(ctx, GenerationRunInput{
		ProjectID:            req.ProjectID,
		BlockID:              &blockID,
		TaskType:             req.TaskType,
		Provider:             modelProfile.Provider,
		Model:                modelProfile.Model,
		Temperature:          modelProfile.Temperature,
		TopP:                 modelProfile.TopP,
		MaxTokens:            modelProfile.MaxTokens,
		ContextWindow:        modelProfile.ContextWindow,
		PromptTemplateID:     promptTemplateID,
		InputContextSnapshot: snapshot,
	})
	if err != nil {
		return preparedGeneration{}, err
	}

	baseURL := ""
	if modelProfile.BaseURL != nil {
		baseURL = *modelProfile.BaseURL
	}

	messages := []ChatMessage{{Role: "system", Content: contextPreview.SystemPrompt}}
	if req.ConversationID != nil {
		conversation, err := h.repo.GetConversation(ctx, *req.ConversationID)
		if err != nil || conversation.ProjectID != req.ProjectID || conversation.BlockID != req.BlockID {
			return preparedGeneration{}, ErrGenerationResourceNotFound
		}
		history, err := h.repo.ListConversationMessages(ctx, *req.ConversationID)
		if err != nil {
			return preparedGeneration{}, err
		}
		historyForRequest := history
		if req.RegenerateMessageID != nil {
			historyForRequest, err = conversationHistoryBeforeRegeneration(history, *req.RegenerateMessageID)
			if err != nil {
				return preparedGeneration{}, err
			}
		} else if req.RetryUserMessageID != nil {
			historyForRequest, err = conversationHistoryBeforeUserRetry(history, *req.RetryUserMessageID)
			if err != nil {
				return preparedGeneration{}, err
			}
		}
		for _, message := range historyForRequest {
			messages = append(messages, ChatMessage{Role: message.Role, Content: conversationMessageContentForGeneration(message)})
		}
		if !req.SkipConversationSave && req.RegenerateMessageID == nil && req.RetryUserMessageID == nil {
			if _, err := h.repo.AppendConversationMessage(ctx, *req.ConversationID, "user", conversationUserContent(req), &run.ID); err != nil {
				return preparedGeneration{}, err
			}
		}
	}
	messages = append(messages, ChatMessage{Role: "user", Content: contextPreview.UserPrompt})

	return preparedGeneration{
		Request:          req,
		ModelProfile:     modelProfile,
		Run:              run,
		Prompt:           prompt,
		Messages:         messages,
		ContextPreview:   contextPreview,
		PromptTemplateID: promptTemplateID,
		BaseURL:          baseURL,
	}, nil
}

func conversationHistoryBeforeRegeneration(history []ConversationMessage, targetAssistantID string) ([]ConversationMessage, error) {
	targetIndex := -1
	for index, message := range history {
		if message.ID == targetAssistantID {
			if message.Role != "assistant" {
				return nil, ErrInvalidGenerationRequest
			}
			targetIndex = index
			break
		}
	}
	if targetIndex < 0 {
		return nil, ErrGenerationResourceNotFound
	}

	sourceUserIndex := -1
	for index := targetIndex - 1; index >= 0; index-- {
		if history[index].Role == "user" {
			sourceUserIndex = index
			break
		}
	}
	if sourceUserIndex < 0 {
		return nil, ErrInvalidGenerationRequest
	}

	return history[:sourceUserIndex], nil
}

func conversationHistoryBeforeUserRetry(history []ConversationMessage, targetUserID string) ([]ConversationMessage, error) {
	for index, message := range history {
		if message.ID != targetUserID {
			continue
		}
		if message.Role != "user" {
			return nil, ErrInvalidGenerationRequest
		}
		if index != len(history)-1 {
			return nil, ErrInvalidGenerationRequest
		}
		return history[:index], nil
	}
	return nil, ErrGenerationResourceNotFound
}

func conversationMessageContentForGeneration(message ConversationMessage) string {
	if message.Role != "user" || len(message.ContextSnapshot) == 0 {
		return message.Content
	}
	var snapshot contextSnapshot
	if err := json.Unmarshal(message.ContextSnapshot, &snapshot); err != nil {
		return message.Content
	}
	originalContent := strings.TrimSpace(snapshot.UserInstruction)
	if originalContent == "" {
		originalContent = "执行 " + snapshot.TaskType
	}
	if strings.TrimSpace(message.Content) != originalContent {
		return message.Content
	}
	if userPrompt := strings.TrimSpace(snapshot.ContextPreview.UserPrompt); userPrompt != "" {
		return userPrompt
	}
	return message.Content
}

func normalizeMessageIDs(messageIDs []string) ([]string, error) {
	if len(messageIDs) == 0 || len(messageIDs) > 200 {
		return nil, ErrInvalidGenerationRequest
	}
	seen := make(map[string]struct{}, len(messageIDs))
	result := make([]string, 0, len(messageIDs))
	for _, id := range messageIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, ErrInvalidGenerationRequest
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func conversationUserContent(req GenerateOnceRequest) string {
	if value := strings.TrimSpace(req.UserInstruction); value != "" {
		return value
	}
	return "执行 " + req.TaskType
}

func writeSSE(c *gin.Context, event GenerateStreamEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = c.Writer.Write([]byte("data: "))
	_, _ = c.Writer.Write(payload)
	_, _ = c.Writer.Write([]byte("\n\n"))
	c.Writer.Flush()
}

func isOpenAICompatibleProvider(provider string) bool {
	switch provider {
	case "openai_compatible", "openai", "openrouter", "deepseek", "moonshot", "siliconflow":
		return true
	default:
		return false
	}
}

func respondGenerationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidGenerationRequest):
		api.RespondError(c, http.StatusBadRequest, "INVALID_GENERATION_REQUEST", err.Error())
	case errors.Is(err, ErrGenerationResourceNotFound):
		api.RespondError(c, http.StatusNotFound, "GENERATION_RESOURCE_NOT_FOUND", "generation resource not found")
	case errors.Is(err, ErrUnsupportedProvider):
		api.RespondError(c, http.StatusBadRequest, "UNSUPPORTED_PROVIDER", "unsupported provider")
	case errors.Is(err, ErrProviderRequestFailed):
		api.RespondError(c, http.StatusBadGateway, "PROVIDER_REQUEST_FAILED", err.Error())
	default:
		api.RespondError(c, http.StatusInternalServerError, "GENERATION_FAILED", "failed to generate text")
	}
}
