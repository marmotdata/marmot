package glossary

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/glossary/importer"
	"github.com/marmotdata/marmot/internal/core/limits"
	"github.com/rs/zerolog/log"
)

var contentTypes = map[importer.Format]string{
	importer.FormatXLSX: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	importer.FormatCSV:  "text/csv; charset=utf-8",
}

func formatParam(r *http.Request) (importer.Format, bool) {
	switch importer.Format(strings.ToLower(r.URL.Query().Get("format"))) {
	case "", importer.FormatXLSX:
		return importer.FormatXLSX, true
	case importer.FormatCSV:
		return importer.FormatCSV, true
	}
	return "", false
}

// localeParam is ?locale=, else the first language of Accept-Language. The
// profile's default locale covers anything its messages lack.
func localeParam(r *http.Request) string {
	if l := r.URL.Query().Get("locale"); l != "" {
		return l
	}
	first, _, _ := strings.Cut(r.Header.Get("Accept-Language"), ",")
	lang, _, _ := strings.Cut(strings.TrimSpace(first), ";")
	primary, _, _ := strings.Cut(lang, "-")
	return strings.ToLower(primary)
}

func (h *Handler) sendSheet(w http.ResponseWriter, r *http.Request, filename string, rows [][]string) {
	format, ok := formatParam(r)
	if !ok {
		common.RespondError(w, http.StatusBadRequest, "format must be xlsx or csv")
		return
	}
	var buf bytes.Buffer
	if err := importer.Write(&buf, format, h.importer.Columns(), h.importer.Texts(localeParam(r)), rows); err != nil {
		log.Error().Err(err).Msg("Failed to write glossary sheet")
		common.RespondError(w, http.StatusInternalServerError, "Failed to write the file")
		return
	}
	w.Header().Set("Content-Type", contentTypes[format])
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, filename, format))
	_, _ = w.Write(buf.Bytes())
}

// @Summary Download the glossary import template
// @Description An XLSX (with drop-downs, comments and a guide sheet) or CSV whose columns are the term's own fields plus the metamodel profile's glossary_term fields. Headers are stable field IDs; labels follow ?locale= or Accept-Language.
// @Tags glossary
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Produce text/csv
// @Param format query string false "xlsx (default) or csv"
// @Param locale query string false "Language for labels and help"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {file} file
// @Failure 400 {object} common.ErrorResponse
// @Router /glossary/import/template [get]
func (h *Handler) importTemplate(w http.ResponseWriter, r *http.Request) {
	h.sendSheet(w, r, "glossary-template", nil)
}

// @Summary Export the glossary
// @Description Every term in the import template's columns, so the file can be edited and imported back with on_existing=update. Cells that a spreadsheet would run as formulas are escaped in CSV.
// @Tags glossary
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Produce text/csv
// @Param format query string false "xlsx (default) or csv"
// @Param locale query string false "Language for labels and help"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {file} file
// @Failure 400 {object} common.ErrorResponse
// @Router /glossary/export [get]
func (h *Handler) exportGlossary(w http.ResponseWriter, r *http.Request) {
	rows, err := h.importer.Export(r.Context(), h.glossaryService)
	if errors.Is(err, importer.ErrTooManyTerms) {
		common.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to export the glossary")
		common.RespondError(w, http.StatusInternalServerError, "Failed to export the glossary")
		return
	}
	h.sendSheet(w, r, "glossary", rows)
}

// @Summary Import glossary terms from a file
// @Description Reads an XLSX or CSV in the template's shape and checks every row. mode=validate (default) writes nothing; mode=apply writes all rows or none, and only when no row has errors. Rows whose name matches a term are skipped unless on_existing=update; on update, empty cells keep the current value.
// @Tags glossary
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "XLSX or CSV file"
// @Param mode query string false "validate (default) or apply"
// @Param on_existing query string false "skip (default) or update"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} importer.Result
// @Failure 400 {object} common.ErrorResponse
// @Failure 413 {object} common.ErrorResponse
// @Failure 422 {object} importer.Result
// @Router /glossary/import [post]
func (h *Handler) importTerms(w http.ResponseWriter, r *http.Request) {
	apply := false
	switch r.URL.Query().Get("mode") {
	case "", "validate":
	case "apply":
		apply = true
	default:
		common.RespondError(w, http.StatusBadRequest, "mode must be validate or apply")
		return
	}
	onExisting := importer.OnExisting(r.URL.Query().Get("on_existing"))
	switch onExisting {
	case "":
		onExisting = importer.OnExistingSkip
	case importer.OnExistingSkip, importer.OnExistingUpdate:
	default:
		common.RespondError(w, http.StatusBadRequest, "on_existing must be skip or update")
		return
	}

	// Room for the multipart envelope around the largest file accepted.
	r.Body = http.MaxBytesReader(w, r.Body, importer.MaxBytes+1<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			common.RespondError(w, http.StatusRequestEntityTooLarge, importer.ErrTooLarge.Error())
			return
		}
		common.RespondError(w, http.StatusBadRequest, "send the file in the multipart field \"file\"")
		return
	}
	defer func() { _ = file.Close() }()
	head := make([]byte, 4)
	n, _ := io.ReadFull(file, head)
	format, err := importer.FormatOf(header.Filename, head[:n])
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	sheet, err := importer.Read(io.MultiReader(bytes.NewReader(head[:n]), file), format)
	switch {
	case errors.Is(err, importer.ErrTooLarge):
		common.RespondError(w, http.StatusRequestEntityTooLarge, err.Error())
		return
	case err != nil:
		common.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.importer.Validate(r.Context(), sheet, onExisting)
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate glossary import")
		common.RespondError(w, http.StatusInternalServerError, "Failed to validate the file")
		return
	}
	if !apply {
		common.RespondJSON(w, http.StatusOK, result)
		return
	}
	if !result.Valid() {
		common.RespondJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	if err := h.importer.Apply(r.Context(), h.glossaryService, result); err != nil {
		if limitErr, ok := limits.AsLimitExceeded(err); ok {
			common.RespondLimitExceeded(w, limitErr)
			return
		}
		if errors.Is(err, glossary.ErrInvalidInput) {
			common.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Msg("Failed to apply glossary import")
		common.RespondError(w, http.StatusInternalServerError, "Failed to import the file; nothing was written")
		return
	}
	common.RespondJSON(w, http.StatusOK, result)
}
