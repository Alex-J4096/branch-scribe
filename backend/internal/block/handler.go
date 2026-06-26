package block

import (
	"errors"
	"net/http"

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
	router.GET("/projects/:projectId/blocks", handler.List)
	router.POST("/projects/:projectId/blocks", handler.Create)
	router.GET("/blocks/:blockId", handler.Get)
	router.PATCH("/blocks/:blockId", handler.Update)
	router.PATCH("/blocks/:blockId/associations", handler.UpdateAssociations)
	router.DELETE("/blocks/:blockId", handler.Delete)
	router.POST("/blocks/:blockId/fork", handler.Fork)

	router.GET("/blocks/:blockId/revisions", handler.ListRevisions)
	router.POST("/blocks/:blockId/revisions", handler.CreateRevision)
	router.GET("/revisions/:revisionId", handler.GetRevision)
	router.POST("/blocks/:blockId/revisions/:revisionId/select", handler.SelectRevision)
}

func (h *Handler) List(c *gin.Context) {
	blocks, err := h.repo.List(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "BLOCK_LIST_FAILED", "failed to list blocks")
		return
	}
	api.RespondOK(c, blocks)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BLOCK_REQUEST", "invalid block request")
		return
	}

	detail, err := h.repo.Create(c.Request.Context(), c.Param("projectId"), req)
	if err != nil {
		respondBlockError(c, err, "BLOCK_CREATE_FAILED", "failed to create block")
		return
	}

	c.JSON(http.StatusCreated, api.Envelope{Data: detail, Error: nil})
}

func (h *Handler) Get(c *gin.Context) {
	detail, err := h.repo.Get(c.Request.Context(), c.Param("blockId"))
	if err != nil {
		respondBlockError(c, err, "BLOCK_GET_FAILED", "failed to get block")
		return
	}
	api.RespondOK(c, detail)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BLOCK_REQUEST", "invalid block request")
		return
	}

	newBlock, err := h.repo.Update(c.Request.Context(), c.Param("blockId"), req)
	if err != nil {
		respondBlockError(c, err, "BLOCK_UPDATE_FAILED", "failed to update block")
		return
	}
	api.RespondOK(c, newBlock)
}

func (h *Handler) UpdateAssociations(c *gin.Context) {
	var req UpdateBlockAssociationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BLOCK_REQUEST", "invalid block association request")
		return
	}

	newBlock, err := h.repo.UpdateAssociations(c.Request.Context(), c.Param("blockId"), req)
	if err != nil {
		respondBlockError(c, err, "BLOCK_ASSOCIATIONS_UPDATE_FAILED", "failed to update block associations")
		return
	}
	api.RespondOK(c, newBlock)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("blockId")); err != nil {
		respondBlockError(c, err, "BLOCK_DELETE_FAILED", "failed to delete block")
		return
	}
	api.RespondOK(c, gin.H{"deleted": true})
}

func (h *Handler) Fork(c *gin.Context) {
	var req ForkBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BLOCK_REQUEST", "invalid block request")
		return
	}

	detail, err := h.repo.Fork(c.Request.Context(), c.Param("blockId"), req)
	if err != nil {
		respondBlockError(c, err, "BLOCK_FORK_FAILED", "failed to fork block")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: detail, Error: nil})
}

func (h *Handler) ListRevisions(c *gin.Context) {
	revisions, err := h.repo.ListRevisions(c.Request.Context(), c.Param("blockId"))
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "REVISION_LIST_FAILED", "failed to list revisions")
		return
	}
	api.RespondOK(c, revisions)
}

func (h *Handler) CreateRevision(c *gin.Context) {
	var req CreateRevisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_REVISION_REQUEST", "invalid revision request")
		return
	}

	revision, err := h.repo.CreateRevision(c.Request.Context(), c.Param("blockId"), req)
	if err != nil {
		respondBlockError(c, err, "REVISION_CREATE_FAILED", "failed to create revision")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: revision, Error: nil})
}

func (h *Handler) GetRevision(c *gin.Context) {
	revision, err := h.repo.GetRevision(c.Request.Context(), c.Param("revisionId"))
	if err != nil {
		respondBlockError(c, err, "REVISION_GET_FAILED", "failed to get revision")
		return
	}
	api.RespondOK(c, revision)
}

func (h *Handler) SelectRevision(c *gin.Context) {
	newBlock, err := h.repo.SelectRevision(c.Request.Context(), c.Param("blockId"), c.Param("revisionId"))
	if err != nil {
		respondBlockError(c, err, "REVISION_SELECT_FAILED", "failed to select revision")
		return
	}
	api.RespondOK(c, newBlock)
}

func respondBlockError(c *gin.Context, err error, code string, message string) {
	switch {
	case errors.Is(err, ErrInvalidBlock), errors.Is(err, ErrInvalidRevision):
		api.RespondError(c, http.StatusBadRequest, "INVALID_BLOCK_REQUEST", err.Error())
	case errors.Is(err, ErrBlockNotFound):
		api.RespondError(c, http.StatusNotFound, "BLOCK_NOT_FOUND", "block not found")
	default:
		api.RespondError(c, http.StatusInternalServerError, code, message)
	}
}
