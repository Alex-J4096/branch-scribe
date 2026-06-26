package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	router.POST("/generate/once", handler.GenerateOnce)
	router.POST("/generate/stream", handler.GenerateStream)
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
		Messages:    []ChatMessage{{Role: "user", Content: prepared.Prompt}},
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
			ModelProfileID:   prepared.Request.ModelProfileID,
			PromptTemplateID: prepared.PromptTemplateID,
		})
		return
	}

	var output string
	for event := range stream {
		switch event.Type {
		case "delta":
			output += event.Content
			writeSSE(c, GenerateStreamEvent{Type: "delta", Content: event.Content})
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
				GenerationRun:    &run,
				Prompt:           prepared.Prompt,
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
		Messages:    []ChatMessage{{Role: "user", Content: prepared.Prompt}},
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
		return GenerateOnceResponse{GenerationRun: run, Prompt: prepared.Prompt, ModelProfileID: prepared.Request.ModelProfileID, PromptTemplateID: prepared.PromptTemplateID}, err
	}

	run, err := h.repo.MarkRunSucceeded(ctx, prepared.Run.ID, result, latencyMS)
	if err != nil {
		return GenerateOnceResponse{}, err
	}
	return GenerateOnceResponse{
		OutputText:       result.Content,
		GenerationRun:    run,
		Prompt:           prepared.Prompt,
		ModelProfileID:   prepared.Request.ModelProfileID,
		PromptTemplateID: prepared.PromptTemplateID,
	}, nil
}

type preparedGeneration struct {
	Request          GenerateOnceRequest
	ModelProfile     ModelProfile
	Run              GenerationRun
	Prompt           string
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

	var blockContext BlockContext
	if req.TaskType == "free_write" {
		blockContext, err = h.repo.GetBlockMetadataContext(ctx, req.ProjectID, req.BlockID)
	} else {
		blockContext, err = h.repo.GetBlockContext(ctx, req.ProjectID, req.BlockID)
	}
	if err != nil {
		return preparedGeneration{}, err
	}

	var template *PromptTemplate
	if req.PromptTemplateID != nil {
		found, err := h.repo.GetPromptTemplate(ctx, req.ProjectID, *req.PromptTemplateID)
		if err != nil {
			return preparedGeneration{}, err
		}
		template = &found
	} else if found, err := h.repo.GetDefaultPromptTemplate(ctx, req.ProjectID, req.TaskType); err == nil {
		template = &found
	} else if !errors.Is(err, ErrGenerationResourceNotFound) {
		return preparedGeneration{}, err
	}

	prompt, snapshot := renderPrompt(req, blockContext, template)
	blockID := req.BlockID
	var promptTemplateID *string
	if template != nil {
		promptTemplateID = &template.ID
	}

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
		Request:          req,
		ModelProfile:     modelProfile,
		Run:              run,
		Prompt:           prompt,
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
