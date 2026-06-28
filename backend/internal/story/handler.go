package story

import (
	"errors"
	"net/http"
	"strings"

	"branchscribe/backend/internal/api"
	"github.com/gin-gonic/gin"
)

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

func RegisterRoutes(router gin.IRouter, h *Handler) {
	router.GET("/projects/:projectId/character-states", h.listCharacterStates)
	router.POST("/projects/:projectId/character-states", h.createCharacterState)
	router.PUT("/character-states/:id", h.updateCharacterState)
	router.DELETE("/character-states/:id", h.deleteCharacterState)
	router.GET("/projects/:projectId/foreshadowings", h.listForeshadowings)
	router.POST("/projects/:projectId/foreshadowings", h.createForeshadowing)
	router.PUT("/foreshadowings/:id", h.updateForeshadowing)
	router.DELETE("/foreshadowings/:id", h.deleteForeshadowing)
	router.GET("/projects/:projectId/timeline-events", h.listTimelineEvents)
	router.POST("/projects/:projectId/timeline-events", h.createTimelineEvent)
	router.PUT("/timeline-events/:id", h.updateTimelineEvent)
	router.DELETE("/timeline-events/:id", h.deleteTimelineEvent)
}

func (h *Handler) listCharacterStates(c *gin.Context) {
	items, err := h.repo.ListCharacterStates(c, c.Param("projectId"), strings.TrimSpace(c.Query("character_id")))
	h.respond(c, items, err)
}
func (h *Handler) createCharacterState(c *gin.Context) {
	var input CharacterStateInput
	if !h.bind(c, &input) {
		return
	}
	item, err := h.repo.CreateCharacterState(c, c.Param("projectId"), input)
	h.respondCreated(c, item, err)
}
func (h *Handler) deleteCharacterState(c *gin.Context) {
	h.respondDeleted(c, h.repo.Delete(c, "character_states", c.Param("id")))
}
func (h *Handler) updateCharacterState(c *gin.Context) {
	var input CharacterStateInput
	if !h.bind(c, &input) {
		return
	}
	item, err := h.repo.UpdateCharacterState(c, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *Handler) listForeshadowings(c *gin.Context) {
	items, err := h.repo.ListForeshadowings(c, c.Param("projectId"), strings.TrimSpace(c.Query("status")))
	h.respond(c, items, err)
}
func (h *Handler) createForeshadowing(c *gin.Context) {
	var input ForeshadowingInput
	if !h.bind(c, &input) {
		return
	}
	item, err := h.repo.CreateForeshadowing(c, c.Param("projectId"), input)
	h.respondCreated(c, item, err)
}
func (h *Handler) updateForeshadowing(c *gin.Context) {
	var input ForeshadowingInput
	if !h.bind(c, &input) {
		return
	}
	item, err := h.repo.UpdateForeshadowing(c, c.Param("id"), input)
	h.respond(c, item, err)
}
func (h *Handler) deleteForeshadowing(c *gin.Context) {
	h.respondDeleted(c, h.repo.Delete(c, "foreshadowings", c.Param("id")))
}

func (h *Handler) listTimelineEvents(c *gin.Context) {
	items, err := h.repo.ListTimelineEvents(c, c.Param("projectId"))
	h.respond(c, items, err)
}
func (h *Handler) createTimelineEvent(c *gin.Context) {
	var input TimelineEventInput
	if !h.bind(c, &input) {
		return
	}
	item, err := h.repo.CreateTimelineEvent(c, c.Param("projectId"), input)
	h.respondCreated(c, item, err)
}
func (h *Handler) deleteTimelineEvent(c *gin.Context) {
	h.respondDeleted(c, h.repo.Delete(c, "timeline_events", c.Param("id")))
}
func (h *Handler) updateTimelineEvent(c *gin.Context) {
	var input TimelineEventInput
	if !h.bind(c, &input) {
		return
	}
	item, err := h.repo.UpdateTimelineEvent(c, c.Param("id"), input)
	h.respond(c, item, err)
}

func (h *Handler) bind(c *gin.Context, value any) bool {
	if err := c.ShouldBindJSON(value); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_STORY_RECORD", "invalid story record request")
		return false
	}
	return true
}
func (h *Handler) respond(c *gin.Context, value any, err error) {
	if h.respondError(c, err) {
		return
	}
	api.RespondOK(c, value)
}
func (h *Handler) respondCreated(c *gin.Context, value any, err error) {
	if h.respondError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: value, Error: nil})
}
func (h *Handler) respondDeleted(c *gin.Context, err error) {
	if h.respondError(c, err) {
		return
	}
	api.RespondOK(c, gin.H{"deleted": true})
}
func (h *Handler) respondError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrInvalidRecord):
		api.RespondError(c, http.StatusBadRequest, "INVALID_STORY_RECORD", err.Error())
	case errors.Is(err, ErrNotFound):
		api.RespondError(c, http.StatusNotFound, "STORY_RECORD_NOT_FOUND", err.Error())
	default:
		api.RespondError(c, http.StatusInternalServerError, "STORY_RECORD_FAILED", "story record operation failed")
	}
	return true
}
