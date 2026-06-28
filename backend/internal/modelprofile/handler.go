package modelprofile

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
	router.GET("/model-profiles", handler.List)
	router.POST("/model-profiles", handler.Create)
	router.GET("/model-profiles/:profileId", handler.Get)
	router.PATCH("/model-profiles/:profileId", handler.Update)
	router.DELETE("/model-profiles/:profileId", handler.Delete)
}

func (h *Handler) List(c *gin.Context) {
	profiles, err := h.repo.List(c.Request.Context())
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "MODEL_PROFILE_LIST_FAILED", "failed to list model profiles")
		return
	}
	api.RespondOK(c, profiles)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateModelProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_MODEL_PROFILE_REQUEST", "invalid model profile request")
		return
	}

	profile, err := h.repo.Create(c.Request.Context(), req)
	if err != nil {
		respondModelProfileError(c, err, "MODEL_PROFILE_CREATE_FAILED", "failed to create model profile")
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: profile, Error: nil})
}

func (h *Handler) Get(c *gin.Context) {
	profile, err := h.repo.Get(c.Request.Context(), c.Param("profileId"))
	if err != nil {
		respondModelProfileError(c, err, "MODEL_PROFILE_GET_FAILED", "failed to get model profile")
		return
	}
	api.RespondOK(c, profile)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateModelProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_MODEL_PROFILE_REQUEST", "invalid model profile request")
		return
	}

	profile, err := h.repo.Update(c.Request.Context(), c.Param("profileId"), req)
	if err != nil {
		respondModelProfileError(c, err, "MODEL_PROFILE_UPDATE_FAILED", "failed to update model profile")
		return
	}
	api.RespondOK(c, profile)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("profileId")); err != nil {
		respondModelProfileError(c, err, "MODEL_PROFILE_DELETE_FAILED", "failed to delete model profile")
		return
	}
	api.RespondOK(c, gin.H{"deleted": true})
}

func respondModelProfileError(c *gin.Context, err error, code string, message string) {
	switch {
	case errors.Is(err, ErrInvalidModelProfile):
		api.RespondError(c, http.StatusBadRequest, "INVALID_MODEL_PROFILE_REQUEST", err.Error())
	case errors.Is(err, ErrModelProfileNotFound):
		api.RespondError(c, http.StatusNotFound, "MODEL_PROFILE_NOT_FOUND", "model profile not found")
	default:
		api.RespondError(c, http.StatusInternalServerError, code, message)
	}
}
