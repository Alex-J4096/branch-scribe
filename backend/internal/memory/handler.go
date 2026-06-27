package memory

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"branchscribe/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo     *Repository
	provider EmbeddingProvider
}

func NewHandler(repo *Repository, provider EmbeddingProvider) *Handler {
	return &Handler{repo: repo, provider: provider}
}

func RegisterRoutes(router gin.IRouter, handler *Handler) {
	router.GET("/projects/:projectId/memory", handler.List)
	router.POST("/projects/:projectId/memory", handler.Create)
	router.POST("/projects/:projectId/memory/search", handler.Search)
	router.POST("/projects/:projectId/memory/reindex", handler.Reindex)
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

func (h *Handler) Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_MEMORY_SEARCH_REQUEST", "invalid memory search request")
		return
	}

	filter := ListFilter{
		SourceType: strings.TrimSpace(req.SourceType),
		ChunkKind:  strings.TrimSpace(req.ChunkKind),
		Tag:        strings.TrimSpace(req.Tag),
		Query:      strings.TrimSpace(req.Query),
	}
	var chunks []Chunk
	var err error
	if strings.EqualFold(strings.TrimSpace(req.Mode), "semantic") {
		profileID := strings.TrimSpace(req.ModelProfileID)
		if filter.Query == "" || profileID == "" {
			api.RespondError(c, http.StatusBadRequest, "INVALID_MEMORY_SEARCH_REQUEST", "semantic search requires q and model_profile_id")
			return
		}
		profile, profileErr := h.repo.GetEmbeddingProfile(c.Request.Context(), c.Param("projectId"), profileID)
		if profileErr != nil {
			respondMemoryError(c, profileErr, "MEMORY_SEARCH_FAILED", "failed to configure semantic search")
			return
		}
		vectors, embedErr := h.provider.Embed(c.Request.Context(), profile, []string{filter.Query})
		if embedErr != nil {
			respondMemoryError(c, embedErr, "MEMORY_SEARCH_FAILED", "failed to embed search query")
			return
		}
		chunks, err = h.repo.SemanticSearch(c.Request.Context(), c.Param("projectId"), vectors[0], filter, req.Limit)
	} else {
		chunks, err = h.repo.List(c.Request.Context(), c.Param("projectId"), filter)
	}
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "MEMORY_SEARCH_FAILED", "failed to search memory chunks")
		return
	}
	api.RespondOK(c, chunks)
}

func (h *Handler) Reindex(c *gin.Context) {
	var req ReindexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_REINDEX_REQUEST", "invalid reindex request")
		return
	}
	projectID := c.Param("projectId")
	profileID := strings.TrimSpace(req.ModelProfileID)
	if profileID == "" {
		api.RespondError(c, http.StatusBadRequest, "INVALID_REINDEX_REQUEST", "model_profile_id is required")
		return
	}
	profile, err := h.repo.GetEmbeddingProfile(c.Request.Context(), projectID, profileID)
	if err != nil {
		respondMemoryError(c, err, "MEMORY_REINDEX_FAILED", "failed to configure embedding provider")
		return
	}
	memories, canon, err := h.repo.ListEmbeddingDocuments(c.Request.Context(), projectID)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "MEMORY_REINDEX_FAILED", "failed to load embedding documents")
		return
	}
	memoryCount, dimensions, err := h.embedDocuments(c.Request.Context(), profile, "memory_chunks", memories)
	if err != nil {
		respondMemoryError(c, err, "MEMORY_REINDEX_FAILED", "failed to reindex memory chunks")
		return
	}
	canonCount, canonDimensions, err := h.embedDocuments(c.Request.Context(), profile, "canon_entities", canon)
	if err != nil {
		respondMemoryError(c, err, "MEMORY_REINDEX_FAILED", "failed to reindex canon entities")
		return
	}
	if dimensions == 0 {
		dimensions = canonDimensions
	}
	api.RespondOK(c, ReindexResult{
		MemoryIndexed: memoryCount,
		CanonIndexed:  canonCount,
		Model:         profile.Model,
		Dimensions:    dimensions,
	})
}

func (h *Handler) embedDocuments(ctx context.Context, profile EmbeddingProfile, table string, documents []EmbeddingDocument) (int, int, error) {
	const batchSize = 32
	dimensions := 0
	for start := 0; start < len(documents); start += batchSize {
		end := min(start+batchSize, len(documents))
		batch := documents[start:end]
		inputs := make([]string, len(batch))
		for index := range batch {
			inputs[index] = batch[index].Text
		}
		vectors, err := h.provider.Embed(ctx, profile, inputs)
		if err != nil {
			return start, dimensions, err
		}
		if len(vectors) > 0 {
			if dimensions == 0 {
				dimensions = len(vectors[0])
			}
			for _, vector := range vectors {
				if len(vector) != dimensions {
					return start, dimensions, errors.New("embedding provider returned inconsistent dimensions")
				}
			}
		}
		if err := h.repo.UpdateEmbeddings(ctx, table, batch, vectors); err != nil {
			return start, dimensions, err
		}
	}
	return len(documents), dimensions, nil
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
	case errors.Is(err, ErrEmbeddingNotConfigured):
		api.RespondError(c, http.StatusBadRequest, "EMBEDDING_NOT_CONFIGURED", "embedding model is not configured for this profile")
	case errors.Is(err, ErrEmbeddingRequestFailed):
		api.RespondError(c, http.StatusBadGateway, "EMBEDDING_PROVIDER_FAILED", err.Error())
	case errors.Is(err, ErrInvalidMemoryChunk):
		api.RespondError(c, http.StatusBadRequest, "INVALID_MEMORY_REQUEST", err.Error())
	case errors.Is(err, ErrMemoryChunkNotFound):
		api.RespondError(c, http.StatusNotFound, "MEMORY_NOT_FOUND", "memory chunk not found")
	default:
		api.RespondError(c, http.StatusInternalServerError, code, message)
	}
}
