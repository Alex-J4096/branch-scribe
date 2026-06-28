package transfer

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"branchscribe/backend/internal/api"
	"github.com/gin-gonic/gin"
)

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

func RegisterRoutes(router gin.IRouter, h *Handler) {
	router.GET("/projects/:projectId/export/markdown", h.exportMarkdown)
	router.GET("/projects/:projectId/backup", h.backup)
	router.POST("/projects/import", h.importBackup)
}

func (h *Handler) exportMarkdown(c *gin.Context) {
	projectID := c.Param("projectId")
	branchID, chapterID := strings.TrimSpace(c.Query("branch_id")), strings.TrimSpace(c.Query("chapter_id"))
	if (branchID == "") == (chapterID == "") {
		api.RespondError(c, http.StatusBadRequest, "INVALID_EXPORT_REQUEST", "provide exactly one of branch_id or chapter_id")
		return
	}
	var document MarkdownDocument
	var err error
	if branchID != "" {
		document, err = h.repo.ExportBranchMarkdown(c, projectID, branchID)
	} else {
		document, err = h.repo.ExportChapterMarkdown(c, projectID, chapterID)
	}
	if h.respondError(c, err) {
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, document.Filename))
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(document.Content))
}

func (h *Handler) backup(c *gin.Context) {
	backup, err := h.repo.Backup(c, c.Param("projectId"))
	if h.respondError(c, err) {
		return
	}
	backup.ExportedAt = time.Now().UTC()
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="branchscribe-%s.json"`, backup.ProjectID))
	c.JSON(http.StatusOK, backup)
}

func (h *Handler) importBackup(c *gin.Context) {
	var backup Backup
	if err := c.ShouldBindJSON(&backup); err != nil {
		api.RespondError(c, http.StatusBadRequest, "INVALID_BACKUP", "invalid backup document")
		return
	}
	if h.respondError(c, h.repo.Import(c, backup)) {
		return
	}
	c.JSON(http.StatusCreated, api.Envelope{Data: gin.H{"project_id": backup.ProjectID}, Error: nil})
}

func (h *Handler) respondError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrInvalidExport):
		api.RespondError(c, http.StatusBadRequest, "INVALID_BACKUP", err.Error())
	case errors.Is(err, ErrNotFound):
		api.RespondError(c, http.StatusNotFound, "EXPORT_RESOURCE_NOT_FOUND", err.Error())
	case errors.Is(err, ErrImportConflict):
		api.RespondError(c, http.StatusConflict, "BACKUP_PROJECT_EXISTS", err.Error())
	default:
		api.RespondError(c, http.StatusInternalServerError, "TRANSFER_FAILED", "project transfer operation failed")
	}
	return true
}
