package prompttemplate

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
	router.GET("/projects/:projectId/prompt-templates", handler.List)
	router.POST("/projects/:projectId/prompt-templates", handler.Create)
	router.GET("/prompt-templates/:templateId", handler.Get)
	router.PATCH("/prompt-templates/:templateId", handler.Update)
	router.DELETE("/prompt-templates/:templateId", handler.Delete)
}

func (h *Handler) List(c *gin.Context) {
	templates, err := h.repo.List(c.Request.Context(), c.Param("projectId"), c.Query("task_type"))
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "PROMPT_TEMPLATE_LIST_FAILED", "failed to list prompt templates")
		return
	}
	api.RespondOK(c, templates)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreatePromptTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_PROMPT_TEMPLATE_REQUEST", "invalid prompt template request")
		return
	}

	template, err := h.repo.Create(c.Request.Context(), c.Param("projectId"), req)
	if err != nil {
		respondPromptTemplateError(c, err, "PROMPT_TEMPLATE_CREATE_FAILED", "failed to create prompt template")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: template, Error: nil})
}

func (h *Handler) Get(c *gin.Context) {
	template, err := h.repo.Get(c.Request.Context(), c.Param("templateId"))
	if err != nil {
		respondPromptTemplateError(c, err, "PROMPT_TEMPLATE_GET_FAILED", "failed to get prompt template")
		return
	}
	api.RespondOK(c, template)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdatePromptTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_PROMPT_TEMPLATE_REQUEST", "invalid prompt template request")
		return
	}

	template, err := h.repo.Update(c.Request.Context(), c.Param("templateId"), req)
	if err != nil {
		respondPromptTemplateError(c, err, "PROMPT_TEMPLATE_UPDATE_FAILED", "failed to update prompt template")
		return
	}
	api.RespondOK(c, template)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("templateId")); err != nil {
		respondPromptTemplateError(c, err, "PROMPT_TEMPLATE_DELETE_FAILED", "failed to delete prompt template")
		return
	}
	api.RespondOK(c, gin.H{"deleted": true})
}

func respondPromptTemplateError(c *gin.Context, err error, code string, message string) {
	switch {
	case errors.Is(err, ErrInvalidPromptTemplate):
		api.RespondError(c, http.StatusBadRequest, "INVALID_PROMPT_TEMPLATE_REQUEST", err.Error())
	case errors.Is(err, ErrPromptTemplateNotFound):
		api.RespondError(c, http.StatusNotFound, "PROMPT_TEMPLATE_NOT_FOUND", "prompt template not found")
	default:
		api.RespondError(c, http.StatusInternalServerError, code, message)
	}
}
