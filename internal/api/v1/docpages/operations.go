package docpages

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
} // @name DocsCreatePageRequest

type UpdatePageRequest struct {
	Title   *string `json:"title,omitempty"`
	Emoji   *string `json:"emoji,omitempty"`
	Content *string `json:"content,omitempty"`
} // @name DocsUpdatePageRequest

type MovePageRequest struct {
	ParentID *string `json:"parent_id,omitempty"`
	Position int     `json:"position"`
} // @name DocsMovePageRequest

type SearchPagesResponse struct {
	Pages []*docs.Page `json:"pages"`
	Total int          `json:"total"`
} // @name DocsSearchPagesResponse

type UploadImageRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        string `json:"data"` // Base64 encoded
} // @name DocsUploadImageRequest

// @Summary List documentation pages
// @Description The documentation pages written for an asset or data product, as a tree, with the entity's storage use. This is the documentation the Docs tab shows; page bodies are Markdown.
// @Tags docs
// @Produce json
// @Param entityType path string true "Kind of entity the pages belong to" Enums(asset, data_product)
// @Param entityId path string true "Asset MRN or data product ID, URL-encoded"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} docs.PageTree
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getDocsEntityPages
// @Router /api/v1/docs/entity/{entityType}/{entityId}/pages [get]
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

// @Summary Create a documentation page
// @Description Adds a page to an asset or data product, at the top level or under parent_id.
// @Tags docs
// @Accept json
// @Produce json
// @Param entityType path string true "Kind of entity the pages belong to" Enums(asset, data_product)
// @Param entityId path string true "Asset MRN or data product ID, URL-encoded"
// @Param request body CreatePageRequest true "The new page"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} docs.Page
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postDocsEntityPages
// @Router /api/v1/docs/entity/{entityType}/{entityId}/pages [post]
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

	createdBy := common.CreatedBy(r.Context())

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

// @Summary Get a documentation page
// @Description One page with its Markdown content.
// @Tags docs
// @Produce json
// @Param pageId path string true "Page ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} docs.Page
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getDocsPage
// @Router /api/v1/docs/pages/{pageId} [get]
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

// @Summary Update a documentation page
// @Description Changes a page's title, emoji or content; fields left out stay as they are.
// @Tags docs
// @Accept json
// @Produce json
// @Param pageId path string true "Page ID"
// @Param request body UpdatePageRequest true "Fields to change"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} docs.Page
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID putDocsPage
// @Router /api/v1/docs/pages/{pageId} [put]
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

// @Summary Delete a documentation page
// @Description Deletes a page along with its child pages and images.
// @Tags docs
// @Param pageId path string true "Page ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID deleteDocsPage
// @Router /api/v1/docs/pages/{pageId} [delete]
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

// @Summary Move a documentation page
// @Description Moves a page under another parent, or to the top level when parent_id is omitted, at the given position.
// @Tags docs
// @Accept json
// @Produce json
// @Param pageId path string true "Page ID"
// @Param request body MovePageRequest true "New parent and position"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} docs.Page
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID putDocsPageMove
// @Router /api/v1/docs/pages/{pageId}/move [put]
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

// @Summary Search documentation pages
// @Description Full-text search across one asset's or data product's pages.
// @Tags docs
// @Produce json
// @Param entityType path string true "Kind of entity the pages belong to" Enums(asset, data_product)
// @Param entityId path string true "Asset MRN or data product ID, URL-encoded"
// @Param q query string false "Search text"
// @Param limit query int false "Maximum number of pages to return" default(20)
// @Param offset query int false "Number of pages to skip" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} SearchPagesResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getDocsEntitySearch
// @Router /api/v1/docs/entity/{entityType}/{entityId}/search [get]
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

	common.RespondJSON(w, http.StatusOK, SearchPagesResponse{Pages: pages, Total: total})
}

// @Summary Get documentation storage use
// @Description Page count, image count and bytes used against the limit for one asset or data product.
// @Tags docs
// @Produce json
// @Param entityType path string true "Kind of entity the pages belong to" Enums(asset, data_product)
// @Param entityId path string true "Asset MRN or data product ID, URL-encoded"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} docs.StorageStats
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getDocsEntityStats
// @Router /api/v1/docs/entity/{entityType}/{entityId}/stats [get]
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

// @Summary List a page's images
// @Tags docs
// @Produce json
// @Param pageId path string true "Page ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} docs.ImageMeta
// @Failure 500 {object} common.ErrorResponse
// @ID getDocsPageImages
// @Router /api/v1/docs/pages/{pageId}/images [get]
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

// @Summary Upload an image to a page
// @Description Takes base64 data or a data URL; JPEG, PNG, GIF or WebP up to 5MB. Reference the returned url from the page's Markdown.
// @Tags docs
// @Accept json
// @Produce json
// @Param pageId path string true "Page ID"
// @Param request body UploadImageRequest true "The image"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} docs.ImageMeta
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postDocsPageImages
// @Router /api/v1/docs/pages/{pageId}/images [post]
func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageId")

	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)

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

// @Summary Get an image
// @Description The image's bytes, with its content type.
// @Tags docs
// @Produce image/jpeg,image/png,image/gif,image/webp
// @Param imageId path string true "Image ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {file} binary
// @Success 304
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getDocsImage
// @Router /api/v1/docs/images/{imageId} [get]
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

	// Images are immutable once created but require auth, so only the requesting browser may cache them
	w.Header().Set("Content-Type", image.ContentType)
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(image.SizeBytes))
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+imageID+`"`)
	w.Header().Set("Last-Modified", image.CreatedAt.UTC().Format(time.RFC1123))

	// Check If-None-Match for caching
	if r.Header.Get("If-None-Match") == `"`+imageID+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image.Data) //nolint:gosec // G705: image is re-encoded on upload, served with CSP default-src 'none' and nosniff
}

// @Summary Delete an image
// @Tags docs
// @Param imageId path string true "Image ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID deleteDocsImage
// @Router /api/v1/docs/images/{imageId} [delete]
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
