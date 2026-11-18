package lineage

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/rs/zerolog/log"
)

// maxOpenLineageBody caps an OpenLineage request, a single event or a batch, after decompression. Large Spark plans can run to a few megabytes, so this leaves room for them while stopping a request from exhausting memory.
const maxOpenLineageBody = 10 << 20

// maxLoggedBody caps how much of an event the debug log repeats.
const maxLoggedBody = 4 << 10

// errBodyTooLarge is a request whose body, decompressed, is over maxOpenLineageBody.
var errBodyTooLarge = errors.New("request body too large")

// errUnsupportedEncoding is a request compressed with something other than gzip.
var errUnsupportedEncoding = errors.New("unsupported content encoding")

// readRequestBody reads a lineage request body, decompressing it when the client sent Content-Encoding: gzip as the OpenLineage clients do with compression turned on, and holding it to maxOpenLineageBody either way.
func readRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	var reader io.Reader = http.MaxBytesReader(w, r.Body, maxOpenLineageBody)
	switch enc := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding"))); enc {
	case "", "identity":
	case "gzip", "x-gzip":
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("reading gzip body: %w", err)
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	default:
		return nil, fmt.Errorf("%w %q, send gzip or no encoding", errUnsupportedEncoding, enc)
	}
	body, err := io.ReadAll(io.LimitReader(reader, maxOpenLineageBody+1))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, errBodyTooLarge
		}
		return nil, fmt.Errorf("reading request body: %w", err)
	}
	if len(body) > maxOpenLineageBody {
		return nil, errBodyTooLarge
	}
	return body, nil
}

// respondBodyError answers a request whose body could not be read.
func respondBodyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errBodyTooLarge):
		common.RespondError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Request body is over the %d MiB limit", maxOpenLineageBody>>20))
	case errors.Is(err, errUnsupportedEncoding):
		common.RespondError(w, http.StatusUnsupportedMediaType, err.Error())
	default:
		common.RespondError(w, http.StatusBadRequest, err.Error())
	}
}

func logBody(body []byte) string {
	if len(body) > maxLoggedBody {
		return string(body[:maxLoggedBody]) + "…"
	}
	return string(body)
}

// @Summary Ingest OpenLineage event
// @Description Accepts one OpenLineage event: a RunEvent, a JobEvent or a DatasetEvent. The body may be gzip-compressed with Content-Encoding: gzip. A malformed event is answered with 400 so the producer does not retry it.
// @Tags lineage
// @Accept json
// @Produce json
// @Param event body lineage.RunEvent true "OpenLineage event"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 "Event processed successfully"
// @Failure 400 {object} common.ErrorResponse
// @Failure 413 {object} common.ErrorResponse
// @Failure 415 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postLineage
// @Router /api/v1/lineage [post]
func (h *Handler) ingestOpenLineageEvent(w http.ResponseWriter, r *http.Request) {
	body, err := readRequestBody(w, r)
	if err != nil {
		respondBodyError(w, err)
		return
	}
	log.Debug().Str("body", logBody(body)).Msg("Received OpenLineage event")

	event, err := lineage.ParseEvent(body)
	if err != nil {
		log.Warn().Err(err).Msg("Rejected OpenLineage event")
		common.RespondError(w, http.StatusBadRequest, "Invalid OpenLineage event: "+err.Error())
		return
	}

	if err := h.processOpenLineageEvent(r, event); err != nil {
		if lineage.IsEventError(err) {
			common.RespondError(w, http.StatusBadRequest, "Invalid OpenLineage event: "+err.Error())
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to process event")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) processOpenLineageEvent(r *http.Request, event *lineage.RunEvent) error {
	log.Info().
		Str("kind", string(event.Kind())).
		Str("event_type", event.EventType).
		Str("run_id", event.Run.RunID).
		Str("job_namespace", event.Job.Namespace).
		Str("job_name", event.Job.Name).
		Int("inputs_count", len(event.Inputs)).
		Int("outputs_count", len(event.Outputs)).
		Str("producer", event.Producer).
		Msg("Processing OpenLineage event")

	if err := h.lineageService.ProcessOpenLineageEvent(r.Context(), event, "openlineage"); err != nil {
		log.Error().Err(err).
			Str("kind", string(event.Kind())).
			Str("event_type", event.EventType).
			Str("run_id", event.Run.RunID).
			Str("job", event.Job.Namespace+"."+event.Job.Name).
			Msg("Failed to process OpenLineage event")
		return err
	}
	return nil
}

// OpenLineageBatchResponse is the reply to a batch of OpenLineage events, in the shape the OpenLineage spec gives its batch endpoint.
type OpenLineageBatchResponse struct {
	// Status is success when every event was processed and partial_success otherwise.
	Status       string                   `json:"status"`
	Summary      OpenLineageBatchSummary  `json:"summary"`
	FailedEvents []OpenLineageFailedEvent `json:"failed_events"`
} // @name OpenLineageBatchResponse

// OpenLineageBatchSummary counts what happened to a batch's events.
type OpenLineageBatchSummary struct {
	Received     int `json:"received"`
	Successful   int `json:"successful"`
	Failed       int `json:"failed"`
	Retriable    int `json:"retriable"`
	NonRetriable int `json:"non_retriable"`
} // @name OpenLineageBatchSummary

// OpenLineageFailedEvent is one event in a batch that was not processed. Retriable is false for an event that will fail the same way however often it is sent.
type OpenLineageFailedEvent struct {
	Index     int    `json:"index"`
	Reason    string `json:"reason"`
	Retriable bool   `json:"retriable"`
} // @name OpenLineageFailedEvent

// isOpenLineageBatch tells an array of OpenLineage events from an array of lineage edges by its first element: every OpenLineage event has an eventTime, and no edge does.
func isOpenLineageBatch(items []json.RawMessage) bool {
	if len(items) == 0 {
		return false
	}
	var first map[string]json.RawMessage
	if err := json.Unmarshal(items[0], &first); err != nil {
		return false
	}
	_, ok := first["eventTime"]
	return ok
}

// ingestOpenLineageBatch processes a batch of OpenLineage events in the order sent, so a run's START is filed before its COMPLETE, and reports each event that failed.
func (h *Handler) ingestOpenLineageBatch(w http.ResponseWriter, r *http.Request, items []json.RawMessage) {
	resp := OpenLineageBatchResponse{Status: "success", FailedEvents: []OpenLineageFailedEvent{}}
	resp.Summary.Received = len(items)
	fail := func(i int, reason string, retriable bool) {
		resp.FailedEvents = append(resp.FailedEvents, OpenLineageFailedEvent{Index: i, Reason: reason, Retriable: retriable})
		resp.Summary.Failed++
		if retriable {
			resp.Summary.Retriable++
		} else {
			resp.Summary.NonRetriable++
		}
	}
	for i, item := range items {
		event, err := lineage.ParseEvent(bytes.TrimSpace(item))
		if err != nil {
			fail(i, "Invalid event: "+err.Error(), false)
			continue
		}
		if err := h.processOpenLineageEvent(r, event); err != nil {
			if lineage.IsEventError(err) {
				fail(i, "Invalid event: "+err.Error(), false)
			} else {
				fail(i, "Server error", true)
			}
			continue
		}
		resp.Summary.Successful++
	}
	if resp.Summary.Failed > 0 {
		resp.Status = "partial_success"
	}
	common.RespondJSON(w, http.StatusOK, resp)
}
