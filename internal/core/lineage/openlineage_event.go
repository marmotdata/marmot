package lineage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EventKind is which of the three OpenLineage event types an event is.
type EventKind string

const (
	// KindRun is a RunEvent: something ran, with a run id, a job and the datasets it read and wrote.
	KindRun EventKind = "run"
	// KindJob is a JobEvent: a job's design-time lineage with no run behind it.
	KindJob EventKind = "job"
	// KindDataset is a DatasetEvent: metadata about one dataset with no job or run.
	KindDataset EventKind = "dataset"
)

// EventError is an OpenLineage event Marmot cannot accept as sent. The API answers it with 400 so the producer does not retry it.
type EventError struct {
	Reason string
}

func (e *EventError) Error() string { return e.Reason }

func eventErrorf(format string, args ...any) error {
	return &EventError{Reason: fmt.Sprintf(format, args...)}
}

// IsEventError reports whether err is a problem with the event itself rather than with processing it.
func IsEventError(err error) bool {
	var e *EventError
	return errors.As(err, &e)
}

// Kind says which OpenLineage event type the event is: the kind ParseEvent worked out, or for an event built in code, a dataset event when it carries a dataset and no job, a job event when it has a job and no run id, and a run event otherwise.
func (e *RunEvent) Kind() EventKind {
	if e.kind != "" {
		return e.kind
	}
	switch {
	case e.Dataset != nil && e.Job.Name == "" && e.Run.RunID == "":
		return KindDataset
	case e.Run.RunID == "" && e.Job.Name != "":
		return KindJob
	default:
		return KindRun
	}
}

// eventTimeLayouts are the timestamp forms producers send: RFC 3339 with an offset, the same without a colon in the offset, and a local time with no offset at all, which is read as UTC.
var eventTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999Z0700",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
}

func parseEventTime(s string) (time.Time, error) {
	for _, layout := range eventTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("eventTime %q is not an ISO 8601 date-time", s)
}

// knownEventTypes are the run states the spec defines.
var knownEventTypes = map[string]bool{
	EventTypeStart: true, EventTypeRunning: true, EventTypeComplete: true,
	EventTypeFail: true, EventTypeAbort: true, EventTypeOther: true,
}

// normaliseEventType upper-cases an event type and turns a missing or unknown one into OTHER, which says nothing about how the run ended.
func normaliseEventType(t string) string {
	t = strings.ToUpper(strings.TrimSpace(t))
	if knownEventTypes[t] {
		return t
	}
	return EventTypeOther
}

// kindFromSchemaURL reads the event type from the end of a schemaURL, which names it as #/$defs/RunEvent or the older #/definitions/RunEvent.
func kindFromSchemaURL(u string) EventKind {
	i := strings.LastIndex(u, "/")
	if i < 0 {
		return ""
	}
	switch u[i+1:] {
	case "RunEvent":
		return KindRun
	case "JobEvent":
		return KindJob
	case "DatasetEvent":
		return KindDataset
	}
	return ""
}

// ParseEvent decodes one OpenLineage event of any of the three types and checks it carries what Marmot needs to file it. The kind comes from the schemaURL when it names one and from the fields present otherwise. The checks follow the spec's required fields, except that producer, schemaURL and eventType may be left out.
func ParseEvent(raw []byte) (*RunEvent, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, eventErrorf("the request body is empty")
	}
	if trimmed[0] != '{' {
		return nil, eventErrorf("an OpenLineage event must be a JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return nil, eventErrorf("the body is not valid JSON: %v", err)
	}
	var event RunEvent
	if err := json.Unmarshal(trimmed, &event); err != nil {
		return nil, eventErrorf("the event does not match the OpenLineage model: %v", err)
	}

	present := func(k string) bool {
		v, ok := fields[k]
		return ok && string(v) != "null"
	}
	kind := kindFromSchemaURL(event.SchemaURL)
	if kind == "" {
		switch {
		case present("run"):
			kind = KindRun
		case present("dataset") && !present("job"):
			kind = KindDataset
		case present("job"):
			kind = KindJob
		default:
			return nil, eventErrorf("the event has no run, job or dataset, so it is not a RunEvent, JobEvent or DatasetEvent")
		}
	}
	event.kind = kind

	if event.rawEventTime == "" {
		return nil, eventErrorf("eventTime is required")
	}
	t, err := parseEventTime(event.rawEventTime)
	if err != nil {
		return nil, &EventError{Reason: err.Error()}
	}
	event.EventTime = t

	switch kind {
	case KindRun:
		if !present("run") || strings.TrimSpace(event.Run.RunID) == "" {
			return nil, eventErrorf("run.runId is required for a RunEvent")
		}
		if err := checkJob(&event.Job, present("job")); err != nil {
			return nil, err
		}
		event.EventType = normaliseEventType(event.EventType)
	case KindJob:
		if err := checkJob(&event.Job, present("job")); err != nil {
			return nil, err
		}
		event.EventType = ""
	case KindDataset:
		if event.Dataset == nil {
			return nil, eventErrorf("dataset is required for a DatasetEvent")
		}
		if err := checkDataset("dataset", event.Dataset); err != nil {
			return nil, err
		}
		event.EventType = ""
	}
	for i := range event.Inputs {
		if err := checkDataset(fmt.Sprintf("inputs[%d]", i), &event.Inputs[i]); err != nil {
			return nil, err
		}
	}
	for i := range event.Outputs {
		if err := checkDataset(fmt.Sprintf("outputs[%d]", i), &event.Outputs[i]); err != nil {
			return nil, err
		}
	}
	return &event, nil
}

func checkJob(job *Job, present bool) error {
	if !present {
		return eventErrorf("job is required")
	}
	if strings.TrimSpace(job.Namespace) == "" {
		return eventErrorf("job.namespace is required")
	}
	if strings.TrimSpace(job.Name) == "" {
		return eventErrorf("job.name is required")
	}
	return nil
}

func checkDataset(where string, d *Dataset) error {
	if strings.TrimSpace(d.Namespace) == "" {
		return eventErrorf("%s.namespace is required", where)
	}
	if strings.TrimSpace(d.Name) == "" {
		return eventErrorf("%s.name is required", where)
	}
	return nil
}
