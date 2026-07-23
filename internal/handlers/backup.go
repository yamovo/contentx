package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/backup"
	"github.com/yamovo/contentx/internal/services"
)

// BackupHandler exposes backup/restore operations over HTTP. All routes are
// admin-only and registered under /api/v1/admin/backup.
type BackupHandler struct {
	mgr        *backup.Manager
	articleSvc *services.ArticleService
}

// NewBackupHandler creates a BackupHandler backed by the given Manager.
// articleSvc is used to rebuild the search index after a DB restore (Round 6 / F2).
func NewBackupHandler(mgr *backup.Manager, articleSvc *services.ArticleService) *BackupHandler {
	return &BackupHandler{mgr: mgr, articleSvc: articleSvc}
}

// Create triggers a backup.
// POST /api/v1/admin/backup?type=db|media|all
//
//	@Summary      Create backup
//	@Description  Creates a database and/or media backup. Admin only.
//	@Tags         Backup
//	@Security     BearerAuth
//	@Produce      json
//	@Param        type  query  string  false  "Backup type"  Enums(db,media,all)  default(all)
//	@Success      200  {object}  APIResponse
//	@Router       /admin/backup [post]
func (h *BackupHandler) Create(c *gin.Context) {
	typ := c.DefaultQuery("type", "all")
	switch typ {
	case "db":
		path, err := h.mgr.Backup()
		if err != nil {
			handleBackupErr(c, err)
			return
		}
		Success(c, gin.H{"type": "db", "path": filepath.Base(path)})
	case "media":
		path, err := h.mgr.BackupMedia()
		if err != nil {
			handleBackupErr(c, err)
			return
		}
		Success(c, gin.H{"type": "media", "path": filepath.Base(path)})
	case "all":
		dbPath, mediaPath, err := h.mgr.BackupAll()
		if err != nil {
			handleBackupErr(c, err)
			return
		}
		resp := gin.H{"type": "all", "db": filepath.Base(dbPath)}
		if mediaPath != "" {
			resp["media"] = filepath.Base(mediaPath)
		}
		Success(c, resp)
	default:
		BadRequest(c, "type must be db, media, or all")
	}
}

// handleBackupErr maps backup.Manager errors to appropriate HTTP status codes.
func handleBackupErr(c *gin.Context, err error) {
	if errors.Is(err, backup.ErrBackupInProgress) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": gin.H{"code": "BACKUP_IN_PROGRESS", "message": err.Error()}})
		return
	}
	if errors.Is(err, backup.ErrSchemaMismatch) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "SCHEMA_MISMATCH", "message": err.Error()}})
		return
	}
	if errors.Is(err, backup.ErrIncompleteBackup) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "INCOMPLETE_BACKUP", "message": err.Error()}})
		return
	}
	Error(c, 500, "BACKUP_FAILED", err.Error())
}

// List returns available backups, newest first.
// GET /api/v1/admin/backup
//
//	@Summary      List backups
//	@Description  Lists available backup files. Admin only.
//	@Tags         Backup
//	@Security     BearerAuth
//	@Produce      json
//	@Success      200  {object}  APIResponse
//	@Router       /admin/backup [get]
func (h *BackupHandler) List(c *gin.Context) {
	list, err := h.mgr.List()
	if err != nil {
		InternalError(c)
		return
	}
	Success(c, list)
}

// Restore restores from a backup file by name.
// POST /api/v1/admin/backup/:file/restore
//
//	@Summary      Restore backup
//	@Description  Restores database or media from a backup file. Admin only. For SQLite the service must be restarted afterwards.
//	@Tags         Backup
//	@Security     BearerAuth
//	@Produce      json
//	@Param        file  path  string  true  "Backup filename"
//	@Success      200  {object}  APIResponse
//	@Router       /admin/backup/{file}/restore [post]
func (h *BackupHandler) Restore(c *gin.Context) {
	name := c.Param("file")
	// Prevent path traversal: only bare filenames are allowed.
	if filepath.Base(name) != name {
		BadRequest(c, "invalid backup filename")
		return
	}
	path := filepath.Join(h.mgr.Dir(), name)

	// Detect backup type by prefix.
	if len(name) >= 6 && name[:6] == "media-" {
		if err := h.mgr.RestoreMedia(path); err != nil {
			handleBackupErr(c, err)
			return
		}
		Success(c, gin.H{"type": "media", "restored": name})
		return
	}

	// Database restore. Capture row counts before restore so we can verify
	// consistency afterwards (pg/mysql only — SQLite closes the connection
	// during restore, so post-restore verification requires a restart).
	var before map[string]int
	if h.mgr.Driver() != "sqlite" {
		if counts, err := h.mgr.RowCounts(); err == nil {
			before = counts
		}
	}

	if err := h.mgr.Restore(path); err != nil {
		handleBackupErr(c, err)
		return
	}

	resp := gin.H{"type": "db", "restored": name}
	// Post-restore row-count verification for pg/mysql.
	if before != nil {
		if after, err := h.mgr.VerifyRowCounts(before); err != nil {
			resp["warning"] = "row count regression detected after restore"
			resp["details"] = err.Error()
		} else {
			resp["row_counts"] = after
		}
	} else if h.mgr.Driver() == "sqlite" {
		resp["warning"] = "sqlite restore completed; restart required to verify row counts; search index will rebuild on restart"
	}

	// Rebuild search index after DB restore (Round 6 / F2).
	// Best-effort: runs in a goroutine so the response is not blocked.
	// For SQLite, the connection is closed during restore so reindex would
	// fail; the index will be rebuilt on restart via the warm-up goroutine.
	if h.articleSvc != nil && h.mgr.Driver() != "sqlite" {
		go func() {
			n, err := h.articleSvc.ReindexAll(context.Background())
			if err != nil {
				slog.Warn("post-restore search reindex failed", "error", err, "backup", name)
				return
			}
			slog.Info("post-restore search index rebuilt", "indexed", n, "backup", name)
		}()
		resp["search_index"] = "rebuilding"
	}

	Success(c, resp)
}

// Download streams a backup file to the client.
// GET /api/v1/admin/backup/:file/download
//
//	@Summary      Download backup
//	@Description  Downloads a backup file. Admin only.
//	@Tags         Backup
//	@Security     BearerAuth
//	@Param        file  path  string  true  "Backup filename"
//	@Success      200  {file}  binary
//	@Failure      400  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /admin/backup/{file}/download [get]
func (h *BackupHandler) Download(c *gin.Context) {
	name := c.Param("file")
	if filepath.Base(name) != name {
		BadRequest(c, "invalid backup filename")
		return
	}
	path := filepath.Join(h.mgr.Dir(), name)
	if _, err := os.Stat(path); err != nil {
		Error(c, 404, "NOT_FOUND", "backup file not found")
		return
	}
	c.FileAttachment(path, name)
}

// Delete removes a backup file by name.
// DELETE /api/v1/admin/backup/:file
//
//	@Summary      Delete backup
//	@Description  Deletes a backup file. Admin only.
//	@Tags         Backup
//	@Security     BearerAuth
//	@Produce      json
//	@Param        file  path  string  true  "Backup filename"
//	@Success      200  {object}  APIResponse
//	@Router       /admin/backup/{file} [delete]
func (h *BackupHandler) Delete(c *gin.Context) {
	name := c.Param("file")
	if err := h.mgr.Delete(name); err != nil {
		Error(c, 500, "DELETE_FAILED", err.Error())
		return
	}
	Success(c, gin.H{"deleted": filepath.Base(name)})
}
