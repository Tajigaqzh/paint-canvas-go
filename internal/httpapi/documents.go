package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"paint-canvas-go/internal/model"
)

const maxDocumentBytes = 16 << 20

type documentInput struct {
	Name     string          `json:"name" binding:"required,max=160"`
	Data     json.RawMessage `json:"data" binding:"required"`
	Revision uint64          `json:"revision"`
}

func listDocuments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 20
		if raw := c.Query("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				fail(c, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
				return
			}
			limit = parsed
		}
		offset := 0
		if raw := c.Query("offset"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				fail(c, http.StatusBadRequest, "invalid_offset", "offset must be zero or greater")
				return
			}
			offset = parsed
		}

		var documents []model.Document
		result := db.WithContext(c.Request.Context()).Select("id", "name", "revision", "created_at", "updated_at").
			Where("user_id = ?", c.GetString(userIDContextKey)).Order("updated_at DESC").
			Limit(limit).Offset(offset).Find(&documents)
		if result.Error != nil {
			fail(c, http.StatusInternalServerError, "database_error", "could not list documents")
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": documents, "limit": limit, "offset": offset})
	}
}

func createDocument(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input documentInput
		if !bindDocument(c, &input) {
			return
		}
		if !validCanvasDocument(input.Data) {
			fail(c, http.StatusBadRequest, "invalid_document", "data must contain activePageId, pageIds, and pages")
			return
		}
		userID := c.GetString(userIDContextKey)
		document := model.Document{
			ID:       uuid.NewString(),
			UserID:   &userID,
			Name:     strings.TrimSpace(input.Name),
			Data:     input.Data,
			Revision: 1,
		}
		if document.Name == "" {
			fail(c, http.StatusBadRequest, "invalid_name", "name must not be empty")
			return
		}
		if err := db.WithContext(c.Request.Context()).Create(&document).Error; err != nil {
			fail(c, http.StatusInternalServerError, "database_error", "could not create document")
			return
		}
		c.JSON(http.StatusCreated, document)
	}
}

func getDocument(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var document model.Document
		if err := db.WithContext(c.Request.Context()).First(&document, "id = ? AND user_id = ?", c.Param("id"), c.GetString(userIDContextKey)).Error; err != nil {
			writeDocumentError(c, err)
			return
		}
		c.JSON(http.StatusOK, document)
	}
}

func updateDocument(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input documentInput
		if !bindDocument(c, &input) {
			return
		}
		if input.Revision == 0 {
			fail(c, http.StatusBadRequest, "revision_required", "revision is required for updates")
			return
		}
		if !validCanvasDocument(input.Data) {
			fail(c, http.StatusBadRequest, "invalid_document", "data must contain activePageId, pageIds, and pages")
			return
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			fail(c, http.StatusBadRequest, "invalid_name", "name must not be empty")
			return
		}
		result := db.WithContext(c.Request.Context()).Model(&model.Document{}).
			Where("id = ? AND user_id = ? AND revision = ?", c.Param("id"), c.GetString(userIDContextKey), input.Revision).
			Updates(map[string]any{"name": name, "data": input.Data, "revision": input.Revision + 1})
		if result.Error != nil {
			fail(c, http.StatusInternalServerError, "database_error", "could not update document")
			return
		}
		if result.RowsAffected == 0 {
			var exists int64
			db.WithContext(c.Request.Context()).Model(&model.Document{}).
				Where("id = ? AND user_id = ?", c.Param("id"), c.GetString(userIDContextKey)).Count(&exists)
			if exists == 0 {
				fail(c, http.StatusNotFound, "document_not_found", "document does not exist")
			} else {
				fail(c, http.StatusConflict, "revision_conflict", "document changed; reload it before saving")
			}
			return
		}
		var document model.Document
		if err := db.WithContext(c.Request.Context()).First(&document, "id = ? AND user_id = ?", c.Param("id"), c.GetString(userIDContextKey)).Error; err != nil {
			writeDocumentError(c, err)
			return
		}
		c.JSON(http.StatusOK, document)
	}
}

func deleteDocument(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := db.WithContext(c.Request.Context()).Delete(&model.Document{}, "id = ? AND user_id = ?", c.Param("id"), c.GetString(userIDContextKey))
		if result.Error != nil {
			fail(c, http.StatusInternalServerError, "database_error", "could not delete document")
			return
		}
		if result.RowsAffected == 0 {
			fail(c, http.StatusNotFound, "document_not_found", "document does not exist")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func bindDocument(c *gin.Context, input *documentInput) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDocumentBytes)
	if err := c.ShouldBindJSON(input); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			fail(c, http.StatusRequestEntityTooLarge, "document_too_large", "document must be 16 MiB or smaller")
		} else {
			fail(c, http.StatusBadRequest, "invalid_request", "request must include a name and valid JSON data")
		}
		return false
	}
	if len(input.Data) == 0 || !json.Valid(input.Data) {
		fail(c, http.StatusBadRequest, "invalid_document", "data must be valid JSON")
		return false
	}
	return true
}

func validCanvasDocument(raw json.RawMessage) bool {
	var document struct {
		ActivePageID string                     `json:"activePageId"`
		PageIDs      []string                   `json:"pageIds"`
		Pages        map[string]json.RawMessage `json:"pages"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return false
	}
	if document.ActivePageID == "" || len(document.PageIDs) == 0 || len(document.Pages) == 0 {
		return false
	}
	if _, exists := document.Pages[document.ActivePageID]; !exists {
		return false
	}
	for _, id := range document.PageIDs {
		if _, exists := document.Pages[id]; !exists {
			return false
		}
	}
	return true
}

func writeDocumentError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fail(c, http.StatusNotFound, "document_not_found", "document does not exist")
		return
	}
	fail(c, http.StatusInternalServerError, "database_error", "could not read document")
}
