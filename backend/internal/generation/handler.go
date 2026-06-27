package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	router.POST("/generate/context-preview", handler.ContextPreview)
	router.POST("/generate/once", handler.GenerateOnce)
	router.POST("/generate/candidates", handler.GenerateCandidates)
	router.POST("/generate/stream", handler.GenerateStream)
	router.POST("/blocks/:blockId/summarize", handler.GenerateBlockSummary)
	router.POST("/branches/:branchId/summarize", handler.GenerateBranchSummary)
	router.POST("/summaries/:summaryId/refresh", handler.RefreshSummary)
	router.GET("/projects/:projectId/summaries", handler.ListSummaries)
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
		candidateRequest.TaskType = "compare_revisions"
		candidateRequest.UserInstruction = strings.TrimSpace(candidateRequest.UserInstruction) +
			fmt.Sprintf("\n生成候选版本 %d。它应与另一个候选在情节选择、表达或节奏上有实质区别，只输出候选正文。", index+1)
		response, err := h.generateOnce(c.Request.Context(), candidateRequest)
		if err != nil {
			respondGenerationError(c, err)
			return
		}
		responses = append(responses, response)
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
	source, err := h.repo.GetBranchSummarySource(c.Request.Context(), req.ProjectID, c.Param("branchId"))
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
	source, err := h.repo.GetSummarySource(c.Request.Context(), req.ProjectID, c.Param("summaryId"))
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
	result, err := h.provider.GenerateOnce(providerCtx, GenerateRequest{
		Provider: profile.Provider,
		Model:    profile.Model,
		BaseURL:  baseURL,
		APIKey:   *profile.APIKey,
		Messages: []ChatMessage{
			{Role: "system", Content: "你是小说编辑。请准确、简洁地概括内容，保留关键人物、事件、因果、地点与未解决冲突，不添加原文没有的信息。只输出摘要正文。"},
			{Role: "user", Content: "内容类型：" + source.TargetType + "\n标题：" + title + "\n\n正文：\n" + content},
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
		})
		return
	}

	var output string
	var reasoning string
	for event := range stream {
		switch event.Type {
		case "delta":
			output += event.Content
			writeSSE(c, GenerateStreamEvent{Type: "delta", Content: event.Content})
		case "reasoning":
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
			})
			return
		case "done":
			latencyMS := int(time.Since(startedAt).Milliseconds())
			run := prepared.Run
			succeededRun, updateErr := h.repo.MarkRunSucceeded(c.Request.Context(), prepared.Run.ID, CompletionResult{
				Content:      output,
				InputTokens:  event.InputTokens,
				OutputTokens: event.OutputTokens,
			}, latencyMS)
			if updateErr != nil {
				writeSSE(c, GenerateStreamEvent{Type: "error", Error: "failed to update generation run"})
				return
			}
			run = succeededRun
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
		}, err
	}

	run, err := h.repo.MarkRunSucceeded(ctx, prepared.Run.ID, result, latencyMS)
	if err != nil {
		return GenerateOnceResponse{}, err
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

	return preparedGeneration{
		Request:      req,
		ModelProfile: modelProfile,
		Run:          run,
		Prompt:       prompt,
		Messages: []ChatMessage{
			{Role: "system", Content: contextPreview.SystemPrompt},
			{Role: "user", Content: contextPreview.UserPrompt},
		},
		ContextPreview:   contextPreview,
		PromptTemplateID: promptTemplateID,
		BaseURL:          baseURL,
	}, nil
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
