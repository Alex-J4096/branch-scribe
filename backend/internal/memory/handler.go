package memory

import (
	"errors"
	"net/http"
	"strings"

	"branchscribe/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func RegisterRoutes(router gin.IRouter, handler *Handler) {
	router.GET("/projects/:projectId/memory", handler.List)
	router.POST("/projects/:projectId/memory", handler.Create)
	router.POST("/blocks/:blockId/memory", handler.CreateFromBlock)
	router.GET("/memory/:memoryId", handler.Get)
	router.PATCH("/memory/:memoryId", handler.Update)
	router.DELETE("/memory/:memoryId", handler.Delete)
}

func (h *Handler) List(c *gin.Context) {
	chunks, err := h.repo.List(c.Request.Context(), c.Param("projectId"), ListFilter{
		SourceType: strings.TrimSpace(c.Query("source_type")),
		ChunkKind:  strings.TrimSpace(c.Query("chunk_kind")),
		Tag:        strings.TrimSpace(c.Query("tag")),
		Query:      strings.TrimSpace(c.Query("q")),
	})
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "MEMORY_LIST_FAILED", "failed to list memory chunks")
		return
	}
	api.RespondOK(c, chunks)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateChunkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_MEMORY_REQUEST", "invalid memory chunk request")
		return
	}

	chunk, err := h.repo.Create(c.Request.Context(), c.Param("projectId"), req)
	if err != nil {
		respondMemoryError(c, err, "MEMORY_CREATE_FAILED", "failed to create memory chunk")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: chunk, Error: nil})
}

func (h *Handler) CreateFromBlock(c *gin.Context) {
	var req CreateFromBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_MEMORY_REQUEST", "invalid memory chunk request")
		return
	}

	chunk, err := h.repo.CreateFromBlock(c.Request.Context(), c.Param("blockId"), req)
	if err != nil {
		respondMemoryError(c, err, "MEMORY_CREATE_FROM_BLOCK_FAILED", "failed to create memory chunk from block")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: chunk, Error: nil})
}

func (h *Handler) Get(c *gin.Context) {
	chunk, err := h.repo.Get(c.Request.Context(), c.Param("memoryId"))
	if err != nil {
		respondMemoryError(c, err, "MEMORY_GET_FAILED", "failed to get memory chunk")
		return
	}
	api.RespondOK(c, chunk)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateChunkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_MEMORY_REQUEST", "invalid memory chunk request")
		return
	}

	chunk, err := h.repo.Update(c.Request.Context(), c.Param("memoryId"), req)
	if err != nil {
		respondMemoryError(c, err, "MEMORY_UPDATE_FAILED", "failed to update memory chunk")
		return
	}
	api.RespondOK(c, chunk)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("memoryId")); err != nil {
		respondMemoryError(c, err, "MEMORY_DELETE_FAILED", "failed to delete memory chunk")
		return
	}
	api.RespondOK(c, gin.H{"deleted": true})
}

func respondMemoryError(c *gin.Context, err error, code string, message string) {
	switch {
	case errors.Is(err, ErrInvalidMemoryChunk):
		api.RespondError(c, http.StatusBadRequest, "INVALID_MEMORY_REQUEST", err.Error())
	case errors.Is(err, ErrMemoryChunkNotFound):
		api.RespondError(c, http.StatusNotFound, "MEMORY_NOT_FOUND", "memory chunk not found")
	default:
		api.RespondError(c, http.StatusInternalServerError, code, message)
	}
}
