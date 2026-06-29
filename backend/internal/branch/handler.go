package branch

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
	router.GET("/projects/:projectId/branches", handler.List)
	router.GET("/branches/:branchId/path", handler.Path)
	router.POST("/projects/:projectId/branches", handler.Create)
	router.POST("/projects/:projectId/branches/fork", handler.Fork)
	router.PATCH("/branches/:branchId", handler.Update)
	router.DELETE("/branches/:branchId", handler.Delete)
}

func (h *Handler) Path(c *gin.Context) {
	path, err := h.repo.Path(c.Request.Context(), c.Param("branchId"))
	if err != nil {
		respondBranchError(c, err, "BRANCH_PATH_FAILED", "failed to get branch path")
		return
	}
	api.RespondOK(c, path)
}

func (h *Handler) List(c *gin.Context) {
	branches, err := h.repo.List(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "BRANCH_LIST_FAILED", "failed to list branches")
		return
	}
	api.RespondOK(c, branches)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BRANCH_REQUEST", "invalid branch request")
		return
	}

	branch, err := h.repo.Create(c.Request.Context(), c.Param("projectId"), req)
	if err != nil {
		respondBranchError(c, err, "BRANCH_CREATE_FAILED", "failed to create branch")
		return
	}

	c.JSON(http.StatusCreated, api.Envelope{Data: branch, Error: nil})
}

func (h *Handler) Fork(c *gin.Context) {
	var req ForkBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BRANCH_REQUEST", "invalid branch request")
		return
	}

	branch, err := h.repo.Fork(c.Request.Context(), c.Param("projectId"), req)
	if err != nil {
		respondBranchError(c, err, "BRANCH_FORK_FAILED", "failed to fork branch")
		return
	}

	c.JSON(http.StatusCreated, api.Envelope{Data: branch, Error: nil})
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BRANCH_REQUEST", "invalid branch request")
		return
	}

	branch, err := h.repo.Update(c.Request.Context(), c.Param("branchId"), req)
	if err != nil {
		respondBranchError(c, err, "BRANCH_UPDATE_FAILED", "failed to update branch")
		return
	}

	api.RespondOK(c, branch)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.repo.Delete(c.Request.Context(), c.Param("branchId")); err != nil {
		respondBranchError(c, err, "BRANCH_DELETE_FAILED", "failed to delete branch")
		return
	}

	api.RespondOK(c, gin.H{"deleted": true})
}

func respondBranchError(c *gin.Context, err error, code string, message string) {
	switch {
	case errors.Is(err, ErrInvalidBranch):
		api.RespondError(c, http.StatusBadRequest, "INVALID_BRANCH_REQUEST", err.Error())
	case errors.Is(err, ErrBranchNotFound):
		api.RespondError(c, http.StatusNotFound, "BRANCH_NOT_FOUND", "branch not found")
	case errors.Is(err, ErrBranchNotEmpty):
		api.RespondError(c, http.StatusConflict, "BRANCH_NOT_EMPTY", err.Error())
	default:
		api.RespondError(c, http.StatusInternalServerError, code, message)
	}
}
