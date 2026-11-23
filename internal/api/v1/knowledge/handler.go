package knowledge

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/knowledge"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	service     knowledge.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(service knowledge.Service, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{service: service, userService: userService, authService: authService, config: config}
}

func (h *Handler) Routes() []common.Route {
	view := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithAuth(h.userService, h.authService, h.config),
		common.RequirePermission(h.userService, "assets", "view"),
	}
	manage := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithAuth(h.userService, h.authService, h.config),
		common.RequirePermission(h.userService, "assets", "manage"),
	}
	return []common.Route{
		{Path: "/api/v1/knowledge/kinds", Method: http.MethodGet, Handler: h.listKinds, Middleware: view},
		{Path: "/api/v1/knowledge/counts", Method: http.MethodGet, Handler: h.counts, Middleware: view},
		{Path: "/api/v1/knowledge/entries", Method: http.MethodGet, Handler: h.listEntries, Middleware: view},
		{Path: "/api/v1/knowledge/entries/{asset_id}", Method: http.MethodGet, Handler: h.getEntry, Middleware: view},
		{Path: "/api/v1/knowledge/entries/{asset_id}", Method: http.MethodPut, Handler: h.saveEntry, Middleware: manage},
		{Path: "/api/v1/knowledge/entries/{asset_id}/reviewed", Method: http.MethodPost, Handler: h.markReviewed, Middleware: manage},
		{Path: "/api/v1/knowledge/entries/{asset_id}/links", Method: http.MethodGet, Handler: h.listLinks, Middleware: view},
		{Path: "/api/v1/knowledge/about/{asset_id}", Method: http.MethodGet, Handler: h.about, Middleware: view},
	}
}

type KindsResponse struct {
	Kinds     []knowledge.Kind     `json:"kinds"`
	Relations []knowledge.Relation `json:"relations"`
} // @name KnowledgeKindsResponse

type EntriesResponse struct {
	Entries []knowledge.Summary `json:"entries"`
	Total   int                 `json:"total"`
} // @name KnowledgeEntriesResponse

// listKinds returns the kinds of knowledge entry, with their statuses, typed fields and starting outline, and the relations an entry can have to other assets.
//
// @Summary  List knowledge kinds
// @Tags     knowledge
// @Produce  json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} KindsResponse
// @ID getKnowledgeKinds
// @Router   /api/v1/knowledge/kinds [get]
func (h *Handler) listKinds(w http.ResponseWriter, _ *http.Request) {
	common.RespondJSON(w, http.StatusOK, KindsResponse{Kinds: knowledge.Kinds(), Relations: knowledge.Relations()})
}

// counts sums knowledge entries by kind and state, and counts those due for review.
//
// @Summary  Count knowledge entries
// @Tags     knowledge
// @Produce  json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} knowledge.Counts
// @ID getKnowledgeCounts
// @Router   /api/v1/knowledge/counts [get]
func (h *Handler) counts(w http.ResponseWriter, r *http.Request) {
	c, err := h.service.Counts(r.Context())
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to count knowledge entries")
		return
	}
	common.RespondJSON(w, http.StatusOK, c)
}

// listEntries lists knowledge entries, most recently changed first.
//
// @Summary  List knowledge entries
// @Tags     knowledge
// @Produce  json
// @Param    kind       query string false "Comma-separated kind keys, such as policy,decision"
// @Param    status     query string false "Comma-separated statuses, such as in_force,accepted"
// @Param    q          query string false "Text to find in the title, summary, fields or pages"
// @Param    review_due query bool   false "Only entries whose review date has passed"
// @Param    mine       query bool   false "Only entries owned by the caller or a team they are in"
// @Param    team       query string false "Comma-separated team IDs; only entries those teams own"
// @Param    sort       query string false "updated (default, or relevance when searching), name, review or relevance"
// @Param    limit      query int    false "Entries per page, at most 100" default(25)
// @Param    offset     query int    false "Entries to skip"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} EntriesResponse
// @ID getKnowledgeEntries
// @Router   /api/v1/knowledge/entries [get]
func (h *Handler) listEntries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := knowledge.ListFilter{
		Kinds:     splitList(q.Get("kind")),
		Statuses:  splitList(q.Get("status")),
		Query:     q.Get("q"),
		ReviewDue: q.Get("review_due") == "true",
		Sort:      q.Get("sort"),
		TeamIDs:   splitList(q.Get("team")),
	}
	if q.Get("mine") == "true" {
		if p, ok := common.PrincipalFromContext(r.Context()); ok {
			f.OwnerID = p.ID()
		}
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	entries, total, err := h.service.List(r.Context(), f)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to list knowledge entries")
		return
	}
	common.RespondJSON(w, http.StatusOK, EntriesResponse{Entries: entries, Total: total})
}

// getEntry returns an asset's knowledge entry.
//
// @Summary  Get a knowledge entry
// @Tags     knowledge
// @Produce  json
// @Param    asset_id path string true "Asset ID of the entry"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} knowledge.Entry
// @Failure  404 {object} common.ErrorResponse
// @ID getKnowledgeEntry
// @Router   /api/v1/knowledge/entries/{asset_id} [get]
func (h *Handler) getEntry(w http.ResponseWriter, r *http.Request) {
	e, err := h.service.Get(r.Context(), r.PathValue("asset_id"))
	if errors.Is(err, knowledge.ErrNotFound) {
		common.RespondError(w, http.StatusNotFound, "No knowledge entry has that asset ID")
		return
	}
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to read the knowledge entry")
		return
	}
	common.RespondJSON(w, http.StatusOK, e)
}

// saveEntry sets an entry's status, typed fields, review date and links, replacing what was there.
//
// @Summary  Save a knowledge entry
// @Tags     knowledge
// @Accept   json
// @Produce  json
// @Param    asset_id path string                       true "Asset ID of the entry"
// @Param    entry    body knowledge.SaveInput true "The entry"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} knowledge.Entry
// @Failure  400 {object} common.ErrorResponse
// @Failure  404 {object} common.ErrorResponse
// @ID putKnowledgeEntry
// @Router   /api/v1/knowledge/entries/{asset_id} [put]
func (h *Handler) saveEntry(w http.ResponseWriter, r *http.Request) {
	var in knowledge.SaveInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	by := ""
	if p, ok := common.PrincipalFromContext(r.Context()); ok {
		by = p.DisplayName()
	}
	e, err := h.service.Save(r.Context(), r.PathValue("asset_id"), in, by)
	respondEntry(w, e, err)
}

type ReviewedRequest struct {
	// ReviewBy is the next review date, as YYYY-MM-DD. Leave it out to keep the current one.
	ReviewBy *string `json:"review_by,omitempty"`
} // @name KnowledgeReviewedRequest

// markReviewed records that an owner checked the entry is still true, and can set the next review date.
//
// @Summary  Mark a knowledge entry reviewed
// @Tags     knowledge
// @Accept   json
// @Produce  json
// @Param    asset_id path string          true  "Asset ID of the entry"
// @Param    request  body ReviewedRequest false "The next review date"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} knowledge.Entry
// @Failure  400 {object} common.ErrorResponse
// @Failure  404 {object} common.ErrorResponse
// @ID postKnowledgeEntryReviewed
// @Router   /api/v1/knowledge/entries/{asset_id}/reviewed [post]
func (h *Handler) markReviewed(w http.ResponseWriter, r *http.Request) {
	var req ReviewedRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}
	by := ""
	if p, ok := common.PrincipalFromContext(r.Context()); ok {
		by = p.DisplayName()
	}
	e, err := h.service.MarkReviewed(r.Context(), r.PathValue("asset_id"), req.ReviewBy, by)
	respondEntry(w, e, err)
}

func respondEntry(w http.ResponseWriter, e *knowledge.Entry, err error) {
	switch {
	case errors.Is(err, knowledge.ErrNotFound):
		common.RespondError(w, http.StatusNotFound, "No knowledge entry has that asset ID")
	case errors.Is(err, knowledge.ErrInvalid):
		common.RespondError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), knowledge.ErrInvalid.Error()+": "))
	case err != nil:
		common.RespondError(w, http.StatusInternalServerError, "Failed to save the knowledge entry")
	default:
		common.RespondJSON(w, http.StatusOK, e)
	}
}

// listLinks pages through an entry's links, by target name.
//
// @Summary  List a knowledge entry's links
// @Tags     knowledge
// @Produce  json
// @Param    asset_id path  string true  "Asset ID of the entry"
// @Param    relation query string false "Only links of this relation"
// @Param    cursor   query string false "The next value of the previous page"
// @Param    limit    query int    false "Links per page, at most 100" default(25)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} knowledge.LinkPage
// @Failure  400 {object} common.ErrorResponse
// @Failure  404 {object} common.ErrorResponse
// @ID getKnowledgeEntryLinks
// @Router   /api/v1/knowledge/entries/{asset_id}/links [get]
func (h *Handler) listLinks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	page, err := h.service.Links(r.Context(), r.PathValue("asset_id"), q.Get("relation"), q.Get("cursor"), limit)
	respond(w, page, err)
}

// about lists the knowledge entries about any asset: linked to it, matched to it by a rule on its tags, type or provider, or linked to something that contains it, such as its schema. Entries in use come first, then those not yet in use, then those no longer in use, each by name.
//
// @Summary  List knowledge about an asset
// @Tags     knowledge
// @Produce  json
// @Param    asset_id path  string true  "Asset ID"
// @Param    kind     query string false "Comma-separated kind keys"
// @Param    state    query string false "Comma-separated states: current, pending, historical"
// @Param    source   query string false "Comma-separated sources: direct, rule, inherited"
// @Param    relation query string false "Only entries linked by this relation"
// @Param    cursor   query string false "The next value of the previous page"
// @Param    limit    query int    false "Entries per page, at most 100" default(25)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} knowledge.AboutPage
// @Failure  400 {object} common.ErrorResponse
// @Failure  404 {object} common.ErrorResponse
// @ID getKnowledgeAbout
// @Router   /api/v1/knowledge/about/{asset_id} [get]
func (h *Handler) about(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := knowledge.AboutFilter{
		Kinds:    splitList(q.Get("kind")),
		Sources:  splitList(q.Get("source")),
		Relation: q.Get("relation"),
		Cursor:   q.Get("cursor"),
	}
	for _, s := range splitList(q.Get("state")) {
		f.States = append(f.States, knowledge.State(s))
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	page, err := h.service.About(r.Context(), r.PathValue("asset_id"), f)
	respond(w, page, err)
}

func respond[T any](w http.ResponseWriter, v *T, err error) {
	switch {
	case errors.Is(err, knowledge.ErrNotFound):
		common.RespondError(w, http.StatusNotFound, "No asset has that ID")
	case errors.Is(err, knowledge.ErrInvalid):
		common.RespondError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), knowledge.ErrInvalid.Error()+": "))
	case err != nil:
		common.RespondError(w, http.StatusInternalServerError, "Failed to read knowledge")
	default:
		common.RespondJSON(w, http.StatusOK, v)
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
