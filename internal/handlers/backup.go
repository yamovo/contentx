package handlers

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/backup"
)

// BackupHandler exposes backup/restore operations over HTTP. All routes are
// admin-only and registered under /api/v1/admin/backup.
type BackupHandler struct {
	mgr *backup.Manager
}

// NewBackupHandler creates a BackupHandler backed by the given Manager.
func NewBackupHandler(mgr *backup.Manager) *BackupHandler {
	return &BackupHandler{mgr: mgr}
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
			Error(c, 500, "BACKUP_FAILED", err.Error())
			return
		}
		Success(c, gin.H{"type": "db", "path": filepath.Base(path)})
	case "media":
		path, err := h.mgr.BackupMedia()
		if err != nil {
			Error(c, 500, "BACKUP_FAILED", err.Error())
			return
		}
		Success(c, gin.H{"type": "media", "path": filepath.Base(path)})
	case "all":
		dbPath, mediaPath, err := h.mgr.BackupAll()
		if err != nil {
			Error(c, 500, "BACKUP_FAILED", err.Error())
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
			Error(c, 500, "RESTORE_FAILED", err.Error())
			return
		}
		Success(c, gin.H{"type": "media", "restored": name})
		return
	}

	// Default: database restore.
	if err := h.mgr.Restore(path); err != nil {
		Error(c, 500, "RESTORE_FAILED", err.Error())
		return
	}
	Success(c, gin.H{"type": "db", "restored": name})
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
