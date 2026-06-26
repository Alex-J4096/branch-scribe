package project

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
	router.GET("/projects", handler.List)
	router.POST("/projects", handler.Create)
	router.GET("/projects/:projectId", handler.Get)
	router.PATCH("/projects/:projectId", handler.Update)
	router.DELETE("/projects/:projectId", handler.Delete)
}

func (h *Handler) List(c *gin.Context) {
	projects, err := h.repo.List(c.Request.Context())
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "PROJECT_LIST_FAILED", "failed to list projects")
		return
	}

	api.RespondOK(c, projects)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_PROJECT_REQUEST", "invalid project request")
		return
	}

	project, err := h.repo.Create(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidProject) {
			api.RespondError(c, http.StatusBadRequest, "INVALID_PROJECT_REQUEST", err.Error())
			return
		}
		api.RespondError(c, http.StatusInternalServerError, "PROJECT_CREATE_FAILED", "failed to create project")
		return
	}

	c.JSON(http.StatusCreated, api.Envelope{Data: project, Error: nil})
}

func (h *Handler) Get(c *gin.Context) {
	project, err := h.repo.Get(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			api.RespondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found")
			return
		}
		api.RespondError(c, http.StatusInternalServerError, "PROJECT_GET_FAILED", "failed to get project")
		return
	}

	api.RespondOK(c, project)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_PROJECT_REQUEST", "invalid project request")
		return
	}

	project, err := h.repo.Update(c.Request.Context(), c.Param("projectId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidProject):
			api.RespondError(c, http.StatusBadRequest, "INVALID_PROJECT_REQUEST", err.Error())
		case errors.Is(err, ErrProjectNotFound):
			api.RespondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found")
		default:
			api.RespondError(c, http.StatusInternalServerError, "PROJECT_UPDATE_FAILED", "failed to update project")
		}
		return
	}

	api.RespondOK(c, project)
}

func (h *Handler) Delete(c *gin.Context) {
	err := h.repo.Delete(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			api.RespondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found")
			return
		}
		api.RespondError(c, http.StatusInternalServerError, "PROJECT_DELETE_FAILED", "failed to delete project")
		return
	}

	api.RespondOK(c, gin.H{"deleted": true})
}
