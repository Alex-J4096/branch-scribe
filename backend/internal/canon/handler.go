package canon

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
	router.GET("/projects/:projectId/canon", handler.List)
	router.POST("/projects/:projectId/canon", handler.Create)
	router.GET("/canon/:entityId", handler.Get)
	router.PATCH("/canon/:entityId", handler.Update)
	router.DELETE("/canon/:entityId", handler.Delete)
}

func (h *Handler) List(c *gin.Context) {
	entities, err := h.repo.List(c.Request.Context(), c.Param("projectId"), ListFilter{
		Type:   strings.TrimSpace(c.Query("type")),
		Status: strings.TrimSpace(c.Query("status")),
		Query:  strings.TrimSpace(c.Query("q")),
	})
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "CANON_LIST_FAILED", "failed to list canon entities")
		return
	}
	api.RespondOK(c, entities)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CANON_REQUEST", "invalid canon entity request")
		return
	}

	entity, err := h.repo.Create(c.Request.Context(), c.Param("projectId"), req)
	if err != nil {
		respondCanonError(c, err, "CANON_CREATE_FAILED", "failed to create canon entity")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: entity, Error: nil})
}

func (h *Handler) Get(c *gin.Context) {
	entity, err := h.repo.Get(c.Request.Context(), c.Param("entityId"))
	if err != nil {
		respondCanonError(c, err, "CANON_GET_FAILED", "failed to get canon entity")
		return
	}
	api.RespondOK(c, entity)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateEntityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_CANON_REQUEST", "invalid canon entity request")
		return
	}

	entity, err := h.repo.Update(c.Request.Context(), c.Param("entityId"), req)
	if err != nil {
		respondCanonError(c, err, "CANON_UPDATE_FAILED", "failed to update canon entity")
		return
	}
	api.RespondOK(c, entity)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("entityId")); err != nil {
		respondCanonError(c, err, "CANON_DELETE_FAILED", "failed to delete canon entity")
		return
	}
	api.RespondOK(c, gin.H{"deleted": true})
}

func respondCanonError(c *gin.Context, err error, code string, message string) {
	switch {
	case errors.Is(err, ErrInvalidCanonEntity):
		api.RespondError(c, http.StatusBadRequest, "INVALID_CANON_REQUEST", err.Error())
	case errors.Is(err, ErrCanonEntityNotFound):
		api.RespondError(c, http.StatusNotFound, "CANON_NOT_FOUND", "canon entity not found")
	default:
		api.RespondError(c, http.StatusInternalServerError, code, message)
	}
}
