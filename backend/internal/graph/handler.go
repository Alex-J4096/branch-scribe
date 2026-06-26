package graph

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
	router.GET("/projects/:projectId/graph", handler.Get)
	router.POST("/projects/:projectId/graph/edges", handler.CreateEdge)
	router.PATCH("/projects/:projectId/graph/nodes/:blockId/position", handler.UpdatePosition)
	router.DELETE("/projects/:projectId/graph/edges/:edgeId", handler.DeleteEdge)
}

func (h *Handler) Get(c *gin.Context) {
	projectGraph, err := h.repo.Get(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "GRAPH_GET_FAILED", "failed to get graph")
		return
	}
	api.RespondOK(c, projectGraph)
}

func (h *Handler) CreateEdge(c *gin.Context) {
	var req CreateEdgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_GRAPH_REQUEST", "invalid graph request")
		return
	}

	edge, err := h.repo.CreateEdge(c.Request.Context(), c.Param("projectId"), req)
	if err != nil {
		respondGraphError(c, err, "GRAPH_EDGE_CREATE_FAILED", "failed to create graph edge")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: edge, Error: nil})
}

func (h *Handler) UpdatePosition(c *gin.Context) {
	var req UpdatePositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_GRAPH_REQUEST", "invalid graph request")
		return
	}

	node, err := h.repo.UpdatePosition(c.Request.Context(), c.Param("projectId"), c.Param("blockId"), req)
	if err != nil {
		respondGraphError(c, err, "GRAPH_NODE_UPDATE_FAILED", "failed to update graph node")
		return
	}
	api.RespondOK(c, node)
}

func (h *Handler) DeleteEdge(c *gin.Context) {
	if err := h.repo.DeleteEdge(c.Request.Context(), c.Param("projectId"), c.Param("edgeId")); err != nil {
		respondGraphError(c, err, "GRAPH_EDGE_DELETE_FAILED", "failed to delete graph edge")
		return
	}
	api.RespondOK(c, gin.H{"deleted": true})
}

func respondGraphError(c *gin.Context, err error, code string, message string) {
	switch {
	case errors.Is(err, ErrInvalidGraph):
		api.RespondError(c, http.StatusBadRequest, "INVALID_GRAPH_REQUEST", err.Error())
	case errors.Is(err, ErrGraphNotFound):
		api.RespondError(c, http.StatusNotFound, "GRAPH_ITEM_NOT_FOUND", "graph item not found")
	default:
		api.RespondError(c, http.StatusInternalServerError, code, message)
	}
}
