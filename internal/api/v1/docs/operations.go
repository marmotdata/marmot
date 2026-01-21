package docs

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/docs"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

type CreatePageRequest struct {
	ParentID *string `json:"parent_id,omitempty"`
	Title    string  `json:"title"`
	Emoji    *string `json:"emoji,omitempty"`
	Content  *string `json:"content,omitempty"`
}

type UpdatePageRequest struct {
	Title   *string `json:"title,omitempty"`
	Emoji   *string `json:"emoji,omitempty"`
	Content *string `json:"content,omitempty"`
}

type MovePageRequest struct {
	ParentID *string `json:"parent_id,omitempty"`
	Position int     `json:"position"`
}

type UploadImageRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        string `json:"data"` // Base64 encoded
}

func (h *Handler) getPageTree(w http.ResponseWriter, r *http.Request) {
	entityType := docs.EntityType(r.PathValue("entityType"))
	encodedEntityID := r.PathValue("entityId")

	entityID, err := url.QueryUnescape(encodedEntityID)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid entity ID format")
		return
	}

	if entityType != docs.EntityTypeAsset && entityType != docs.EntityTypeDataProduct {
		common.RespondError(w, http.StatusBadRequest, "Invalid entity type")
		return
	}

	tree, err := h.docsService.GetPageTree(r.Context(), entityType, entityID)
	if err != nil {
		log.Error().Err(err).Str("entity_type", string(entityType)).Str("entity_id", entityID).Msg("Failed to get page tree")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get page tree")
		return
	}

	common.RespondJSON(w, http.StatusOK, tree)
}

func (h *Handler) createPage(w http.ResponseWriter, r *http.Request) {
	entityType := docs.EntityType(r.PathValue("entityType"))
	encodedEntityID := r.PathValue("entityId")

	entityID, err := url.QueryUnescape(encodedEntityID)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid entity ID format")
		return
	}

	if entityType != docs.EntityTypeAsset && entityType != docs.EntityTypeDataProduct {
		common.RespondError(w, http.StatusBadRequest, "Invalid entity type")
		return
	}

	var req CreatePageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var createdBy *string
	if usr, ok := r.Context().Value(common.UserContextKey).(*user.User); ok && usr != nil {
		createdBy = &usr.ID
	}

	input := docs.CreatePageInput{
		ParentID: req.ParentID,
		Title:    req.Title,
		Emoji:    req.Emoji,
		Content:  req.Content,
	}

	page, err := h.docsService.CreatePage(r.Context(), entityType, entityID, input, createdBy)
	if err != nil {
		if errors.Is(err, docs.ErrMaxPagesExceeded) {
			common.RespondError(w, http.StatusBadRequest, "Maximum pages limit exceeded")
			return
		}
		log.Error().Err(err).Msg("Failed to create page")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create page")
		return
	}

	common.RespondJSON(w, http.StatusCreated, page)
}

func (h *Handler) getPage(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")

	page, err := h.docsService.GetPage(r.Context(), pageID)
	if err != nil {
		if errors.Is(err, docs.ErrPageNotFound) {
			common.RespondError(w, http.StatusNotFound, "Page not found")
			return
		}
		log.Error().Err(err).Str("page_id", pageID).Msg("Failed to get page")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get page")
		return
	}

	common.RespondJSON(w, http.StatusOK, page)
}

func (h *Handler) updatePage(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")

	var req UpdatePageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	input := docs.UpdatePageInput{
		Title:   req.Title,
		Emoji:   req.Emoji,
		Content: req.Content,
	}

	if usr, ok := r.Context().Value(common.UserContextKey).(*user.User); ok && usr != nil {
		input.UpdatedByID = usr.ID
		input.UpdatedByName = usr.Name
	}

	page, err := h.docsService.UpdatePage(r.Context(), pageID, input)
	if err != nil {
		if errors.Is(err, docs.ErrPageNotFound) {
			common.RespondError(w, http.StatusNotFound, "Page not found")
			return
		}
		if errors.Is(err, docs.ErrInvalidInput) {
			common.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Str("page_id", pageID).Msg("Failed to update page")
		common.RespondError(w, http.StatusInternalServerError, "Failed to update page")
		return
	}

	common.RespondJSON(w, http.StatusOK, page)
}

func (h *Handler) deletePage(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")

	err := h.docsService.DeletePage(r.Context(), pageID)
	if err != nil {
		if errors.Is(err, docs.ErrPageNotFound) {
			common.RespondError(w, http.StatusNotFound, "Page not found")
			return
		}
		log.Error().Err(err).Str("page_id", pageID).Msg("Failed to delete page")
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete page")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) movePage(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")

	var req MovePageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	input := docs.MovePageInput{
		ParentID: req.ParentID,
		Position: req.Position,
	}

	page, err := h.docsService.MovePage(r.Context(), pageID, input)
	if err != nil {
		if errors.Is(err, docs.ErrPageNotFound) {
			common.RespondError(w, http.StatusNotFound, "Page not found")
			return
		}
		log.Error().Err(err).Str("page_id", pageID).Msg("Failed to move page")
		common.RespondError(w, http.StatusInternalServerError, "Failed to move page")
		return
	}

	common.RespondJSON(w, http.StatusOK, page)
}

func (h *Handler) searchPages(w http.ResponseWriter, r *http.Request) {
	entityType := docs.EntityType(r.PathValue("entityType"))
	encodedEntityID := r.PathValue("entityId")

	entityID, err := url.QueryUnescape(encodedEntityID)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid entity ID format")
		return
	}

	query := r.URL.Query().Get("q")

	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	pages, total, err := h.docsService.SearchPages(r.Context(), entityType, entityID, query, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to search pages")
		common.RespondError(w, http.StatusInternalServerError, "Failed to search pages")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"pages": pages,
		"total": total,
	})
}

func (h *Handler) getStorageStats(w http.ResponseWriter, r *http.Request) {
	entityType := docs.EntityType(r.PathValue("entityType"))
	encodedEntityID := r.PathValue("entityId")

	entityID, err := url.QueryUnescape(encodedEntityID)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid entity ID format")
		return
	}

	stats, err := h.docsService.GetStorageStats(r.Context(), entityType, entityID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get storage stats")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get storage stats")
		return
	}

	common.RespondJSON(w, http.StatusOK, stats)
}

func (h *Handler) listPageImages(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")

	images, err := h.docsService.ListPageImages(r.Context(), pageID)
	if err != nil {
		log.Error().Err(err).Str("page_id", pageID).Msg("Failed to list page images")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list images")
		return
	}

	common.RespondJSON(w, http.StatusOK, images)
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")

	var req UploadImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Handle data URLs (e.g., "data:image/png;base64,...")
	dataStr := req.Data
	if strings.Contains(dataStr, ",") {
		parts := strings.SplitN(dataStr, ",", 2)
		if len(parts) == 2 {
			dataStr = parts[1]
		}
	}

	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid base64 data")
		return
	}

	input := docs.UploadImageInput{
		Filename:    req.Filename,
		ContentType: req.ContentType,
		Data:        data,
	}

	imageMeta, err := h.docsService.UploadImage(r.Context(), pageID, input)
	if err != nil {
		if errors.Is(err, docs.ErrPageNotFound) {
			common.RespondError(w, http.StatusNotFound, "Page not found")
			return
		}
		if errors.Is(err, docs.ErrImageTooLarge) {
			common.RespondError(w, http.StatusBadRequest, "Image exceeds maximum size (5MB)")
			return
		}
		if errors.Is(err, docs.ErrInvalidImageType) {
			common.RespondError(w, http.StatusBadRequest, "Invalid image type. Allowed: JPEG, PNG, GIF, WebP")
			return
		}
		if errors.Is(err, docs.ErrStorageLimitExceeded) {
			common.RespondError(w, http.StatusBadRequest, "Storage limit exceeded")
			return
		}
		log.Error().Err(err).Str("page_id", pageID).Msg("Failed to upload image")
		common.RespondError(w, http.StatusInternalServerError, "Failed to upload image")
		return
	}

	common.RespondJSON(w, http.StatusCreated, imageMeta)
}

func (h *Handler) getImage(w http.ResponseWriter, r *http.Request) {
	imageID := r.PathValue("imageId")

	image, err := h.docsService.GetImage(r.Context(), imageID)
	if err != nil {
		if errors.Is(err, docs.ErrImageNotFound) {
			common.RespondError(w, http.StatusNotFound, "Image not found")
			return
		}
		log.Error().Err(err).Str("image_id", imageID).Msg("Failed to get image")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get image")
		return
	}

	// Set cache headers (images are immutable once created)
	w.Header().Set("Content-Type", image.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(image.SizeBytes))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+imageID+`"`)
	w.Header().Set("Last-Modified", image.CreatedAt.UTC().Format(time.RFC1123))

	// Check If-None-Match for caching
	if r.Header.Get("If-None-Match") == `"`+imageID+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image.Data)
}

func (h *Handler) deleteImage(w http.ResponseWriter, r *http.Request) {
	imageID := r.PathValue("imageId")

	err := h.docsService.DeleteImage(r.Context(), imageID)
	if err != nil {
		if errors.Is(err, docs.ErrImageNotFound) {
			common.RespondError(w, http.StatusNotFound, "Image not found")
			return
		}
		log.Error().Err(err).Str("image_id", imageID).Msg("Failed to delete image")
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete image")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
