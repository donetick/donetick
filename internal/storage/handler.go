package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"donetick.com/core/config"
	auth "donetick.com/core/internal/auth"
	chRepo "donetick.com/core/internal/chore/repo"
	cRepo "donetick.com/core/internal/circle/repo"
	errorx "donetick.com/core/internal/error"
	storageModel "donetick.com/core/internal/storage/model"
	storageRepo "donetick.com/core/internal/storage/repo"
	user "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler handles file storage-related routes
type Handler struct {
	storage     Storage
	signer      URLSigner
	storageRepo *storageRepo.StorageRepository
	choreRepo   *chRepo.ChoreRepository
	circleRepo  *cRepo.CircleRepository
	maxFileSize int64
}

// NewHandler creates a new Handler
func NewHandler(storage Storage, choreRepo *chRepo.ChoreRepository, circleRepo *cRepo.CircleRepository,
	repo *storageRepo.StorageRepository, signer URLSigner, cfg *config.Config) *Handler {
	return &Handler{storage: storage, circleRepo: circleRepo,
		choreRepo:   choreRepo,
		storageRepo: repo,
		signer:      signer,
		maxFileSize: cfg.Storage.MaxFileSize,
	}
}

// AssetHandler serves signed asset URLs from local storage
func (h *Handler) AssetHandler(c *gin.Context) {

	rawURL := c.Param("filepath")
	logger := logging.FromContext(c)
	logger.Debug("AssetHandler", "url", rawURL)

	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing asset url"})
		return
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url format"})
		return
	}

	sig := c.Query("sig")

	if !h.signer.IsValid(parsed.Path[1:], sig) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid or expired signature for url: " + parsed.Path[1:]})
		return
	}

	filename := parsed.Path[1:] // remove leading slash

	file, err := h.storage.Get(context.Background(), filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	defer file.Close()

	// Detect content type
	// buf := make([]byte, 512)
	// n, _ := file.Read(buf)
	// contentType := http.DetectContentType(buf[:n])

	// Reset reader to stream full file

	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
	// 	return
	// }

	// Set headers
	// c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=604800, immutable")
	c.Header("Expires", time.Now().UTC().Add(7*24*time.Hour).Format(http.TimeFormat))
	c.Status(http.StatusOK)

	// Serve content
	io.Copy(c.Writer, file)
}

func (h *Handler) ChoreUploadHandler(c *gin.Context) {
	// read chore from formdata chore and the file from file:
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		log.Error("failed to get current user from context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		log.Error("failed to get file from formdata", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}

	// validate file size:
	if file.Size > h.maxFileSize {
		log.Error("file size is too large", "size", file.Size)
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file size is too large"})
		return
	}

	entityType, entityID, draftID, _ := handleEntityType(c, h, currentUser)

	// save the file to storage:
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
		return
	}
	defer src.Close()
	uuid := uuid.New().String()

	// append the file extension to the uuid:
	ext := file.Filename[strings.LastIndex(file.Filename, "."):]
	path := fmt.Sprintf("users/%d/%s%s", currentUser.ID, uuid, ext)
	mediaRecord := &storageModel.StorageFile{
		FilePath:   path,
		FileName:   file.Filename,
		SizeBytes:  int(file.Size),
		UserID:     currentUser.ID,
		EntityID:   entityID,
		EntityType: entityType,
		DraftID:    draftID,
	}

	// Save the file first; only write the DB record if the upload succeeds
	// so we never end up with a dangling record pointing at a missing file.
	if err = h.storage.Save(context.Background(), path, src); err != nil {
		log.Error("failed to save file to storage", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	if err := h.storageRepo.AddMediaRecord(c, mediaRecord, currentUser); err != nil {
		// Best-effort cleanup: delete the file we just uploaded.
		if delErr := h.storage.Delete(context.Background(), []string{path}); delErr != nil {
			log.Error("failed to delete orphaned file after DB error", "path", path, "error", delErr)
		}
		switch {
		case err == errorx.ErrNotEnoughSpace:
			log.Error("user has no enough space", "error", err)
			c.JSON(http.StatusInsufficientStorage, gin.H{"error": "no enough space"})
			return
		case err == errorx.ErrNotAPlusMember:
			log.Error("user is not a plus member", "error", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "user is not a plus member"})
			return
		default:
			log.Error("failed to save file record to db", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file record"})
		}
		return
	}
	// generate a signed url for the file:
	signedURL, err := h.signer.Sign(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign url"})
		return
	}
	// return the signed url:
	c.JSON(http.StatusOK, gin.H{
		"path":       path,
		"sign":       signedURL,
		"file_name":  file.Filename,
		"size_bytes": int(file.Size),
	})
}

// handleEntityType returns (entityType, entityID, draftID, ok).
func handleEntityType(c *gin.Context, h *Handler, currentUser *user.UserDetails) (storageModel.EntityType, int, string, bool) {
	log := logging.FromContext(c)
	entityType := c.PostForm("entityType")

	switch entityType {
	case "chore_attachment_draft":
		draftID := c.PostForm("draftId")
		if draftID == "" {
			log.Error("missing draftId for chore_attachment_draft")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing draftId"})
			return storageModel.EntityTypeUnknown, 0, "", false
		}
		return storageModel.EntityTypeChoreAttachmentDraft, 0, draftID, true
	}

	rawEntityId := c.PostForm("entityId")
	if rawEntityId == "" {
		return storageModel.EntityTypeChoreDescription, 0, "", false
	}
	entityID, err := strconv.Atoi(rawEntityId)
	if err != nil {
		log.Error("failed to parse chore ID", "error", err)
		return storageModel.EntityTypeChoreDescription, 0, "", false
	}

	switch entityType {
	case "chore":
		chore, err := h.choreRepo.GetChore(c, entityID, currentUser.ID, currentUser.CircleID)
		if err != nil {
			log.Error("failed to get chore from db", "error", err)
			return storageModel.EntityTypeChoreDescription, 0, "", false
		}
		circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
		if err != nil {
			log.Error("failed to get circle users from db", "error", err)
			return storageModel.EntityTypeChoreDescription, 0, "", false
		}
		now := time.Now().UTC()
		if err := chore.CanEdit(currentUser.ID, circleUsers, &now); err != nil {
			log.Error("user is not allowed to edit chore", "error", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "user is not allowed to edit chore"})
			return storageModel.EntityTypeChoreDescription, 0, "", false
		}
		return storageModel.EntityTypeChoreDescription, chore.ID, "", true
	case "chore_attachment":
		chore, err := h.choreRepo.GetChore(c, entityID, currentUser.ID, currentUser.CircleID)
		if err != nil {
			log.Error("failed to get chore from db", "error", err)
			return storageModel.EntityTypeUnknown, 0, "", false
		}
		circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
		if err != nil {
			log.Error("failed to get circle users from db", "error", err)
			return storageModel.EntityTypeUnknown, 0, "", false
		}
		now := time.Now().UTC()
		if err := chore.CanEdit(currentUser.ID, circleUsers, &now); err != nil {
			log.Error("user is not allowed to edit chore", "error", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "user is not allowed to edit chore"})
			return storageModel.EntityTypeUnknown, 0, "", false
		}
		return storageModel.EntityTypeChoreAttachment, chore.ID, "", true
	default:
		log.Error("invalid entity type", "entityType", entityType)
		return storageModel.EntityTypeUnknown, 0, "", false
	}
}

func (h *Handler) ListChoreAttachmentsHandler(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rawChoreID := c.Param("id")
	choreID, err := strconv.Atoi(rawChoreID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chore id"})
		return
	}

	if _, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID); err != nil {
		log.Error("failed to get chore", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "chore not found"})
		return
	}

	files, err := h.storageRepo.GetAllFilesByOwnerType(c, storageModel.EntityTypeChoreAttachment, choreID)
	if err != nil {
		log.Error("failed to get attachments", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get attachments"})
		return
	}

	type attachmentResponse struct {
		FilePath  string `json:"file_path"`
		FileName  string `json:"file_name"`
		SizeBytes int    `json:"size_bytes"`
		CreatedAt int    `json:"created_at"`
		Sign      string `json:"sign"`
	}

	result := make([]attachmentResponse, 0, len(files))
	for _, f := range files {
		signedURL, err := h.signer.Sign(f.FilePath)
		if err != nil {
			log.Error("failed to sign url", "path", f.FilePath, "error", err)
			continue
		}
		result = append(result, attachmentResponse{
			FilePath:  f.FilePath,
			FileName:  f.FileName,
			SizeBytes: f.SizeBytes,
			CreatedAt: f.CreatedAt,
			Sign:      signedURL,
		})
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) DeleteChoreAttachmentHandler(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rawChoreID := c.Param("id")
	choreID, err := strconv.Atoi(rawChoreID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chore id"})
		return
	}

	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		log.Error("failed to get chore", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "chore not found"})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Error("failed to get circle users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	now := time.Now().UTC()
	if err := chore.CanEdit(currentUser.ID, circleUsers, &now); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed to edit chore"})
		return
	}

	var req struct {
		FilePath string `json:"file_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file_path"})
		return
	}

	file, err := h.storageRepo.GetFileByPath(c, req.FilePath, currentUser.ID)
	if err != nil {
		log.Error("failed to find file record", "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "attachment not found"})
		return
	}

	if file.EntityType != storageModel.EntityTypeChoreAttachment || file.EntityID != choreID {
		c.JSON(http.StatusForbidden, gin.H{"error": "attachment does not belong to this chore"})
		return
	}

	if err := h.storage.Delete(context.Background(), []string{file.FilePath}); err != nil {
		log.Error("failed to delete file from storage", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file"})
		return
	}

	if err := h.storageRepo.RemoveFileRecords(c, []*storageModel.StorageFile{file}, currentUser.ID); err != nil {
		log.Error("failed to remove file record", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove attachment record"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "attachment deleted"})
}

// Routes registers storage-related routes
func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {

	uploadRoutes := r.Group("api/v1/assets")
	uploadRoutes.Use(auth.MiddlewareFunc())
	{
		uploadRoutes.POST("/chore", h.ChoreUploadHandler)
	}

	choreRoutes := r.Group("api/v1/chores")
	choreRoutes.Use(auth.MiddlewareFunc())
	{
		choreRoutes.GET("/:id/attachments", h.ListChoreAttachmentsHandler)
		choreRoutes.DELETE("/:id/attachments", h.DeleteChoreAttachmentHandler)
	}

	// wildcard asset-serving route — must be registered last and on its own group
	assetRoutes := r.Group("api/v1/assets")
	assetRoutes.GET("/*filepath", h.AssetHandler)

}
