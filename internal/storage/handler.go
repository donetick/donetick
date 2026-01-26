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

	//Detect content type
	// buf := make([]byte, 512)
	// n, _ := file.Read(buf)
	// contentType := http.DetectContentType(buf[:n])

	//Reset reader to stream full file

	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
	// 	return
	// }

	// Set headers
	// c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=604800, immutable")
	c.Header("Expires", time.Now().Add(7*24*time.Hour).UTC().Format(http.TimeFormat))
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

	entityType, entityID, _ := handleEntityType(c, h, currentUser)

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
		SizeBytes:  int(file.Size),
		UserID:     currentUser.ID,
		EntityID:   entityID,
		EntityType: entityType,
	}

	if err := h.storageRepo.AddMediaRecord(c,
		mediaRecord,
		currentUser,
	); err != nil {
		if err == errorx.ErrNotEnoughSpace {
			log.Error("user has no enough space", "error", err)
			c.JSON(http.StatusInsufficientStorage, gin.H{"error": "no enough space"})
			return
		} else if err == errorx.ErrNotAPlusMember {
			log.Error("user is not a plus member", "error", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "user is not a plus member"})
			return
		} else {
			log.Error("failed to save file record to db", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file record"})
		}
		return
	}
	err = h.storage.Save(context.Background(), path, src)
	if err != nil {
		log.Error("failed to save file to storage", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}
	// generate a signed url for the file:
	signedURL, err := h.signer.Sign(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign url"})
		return
	}
	// return the signed url:
	c.JSON(http.StatusOK, gin.H{"path": path, "sign": signedURL})
}

func handleEntityType(c *gin.Context, h *Handler, currentUser *user.UserDetails) (storageModel.EntityType, int, bool) {
	log := logging.FromContext(c)
	entityType := c.PostForm("entityType")

	rawEntityId := c.PostForm("entityId")
	if rawEntityId == "" {
		return storageModel.EntityTypeChoreDescription, 0, false
	}
	entityID, err := strconv.Atoi(rawEntityId)
	if err != nil {
		log.Error("failed to parse chore ID", "error", err)
		return storageModel.EntityTypeChoreDescription, 0, false
	}
	switch entityType {
	case "chore":
		chore, err := h.choreRepo.GetChore(c, entityID, currentUser.ID)
		if err != nil {
			log.Error("failed to get chore from db", "error", err)
			return storageModel.EntityTypeChoreDescription, 0, false
		}
		circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
		if err != nil {
			log.Error("failed to get circle users from db", "error", err)
			return storageModel.EntityTypeChoreDescription, 0, false
		}
		now := time.Now().UTC()
		if err := chore.CanEdit(currentUser.ID, circleUsers, &now); err != nil {
			log.Error("user is not allowed to edit chore", "error", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "user is not allowed to edit chore"})
			return storageModel.EntityTypeChoreDescription, 0, false
		}
		return storageModel.EntityTypeChoreDescription, chore.ID, true
	default:
		log.Error("invalid entity type", "entityType", entityType)
		return storageModel.EntityTypeUnknown, 0, false

	}
}

// Routes registers storage-related routes
func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {

	choreAssetRoutes := r.Group("api/v1/assets")
	choreAssetRoutes.Use(auth.MiddlewareFunc())
	{
		// catch all requests to /assets/* and pass them to the AssetHandler
		choreAssetRoutes.POST("/chore", h.ChoreUploadHandler)

	}
	assetRoutes := r.Group("api/v1/assets")
	assetRoutes.GET("/*filepath", h.AssetHandler)

}
