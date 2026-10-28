package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

// Documents are resources people read and write by hand as well as through the UI, such as roles and access policies. They travel as JSON or YAML; YAML is converted to JSON before decoding, so the json tags on the Go types are the one definition of the wire format in both.

// maxDocumentBytes bounds a document body. Policies and roles are small; anything larger is a mistake or an attack.
const maxDocumentBytes = 1 << 20

// DocumentFieldError locates one problem in a document, so an editor can mark the line it is on.
type DocumentFieldError struct {
	// Path is a JSONPath-style location such as bindings[2].members[0], or empty for the document as a whole.
	Path    string `json:"path"`
	Message string `json:"message"`
} // @name DocumentFieldError

// DocumentErrorResponse is the error body for a document that could not be accepted, listing every problem found rather than only the first.
type DocumentErrorResponse struct {
	Error  string               `json:"error"`
	Fields []DocumentFieldError `json:"fields,omitempty"`
} // @name DocumentErrorResponse

// ErrDocumentInvalid wraps a body that is not a well-formed document of the expected shape.
var ErrDocumentInvalid = errors.New("invalid document")

func isYAMLType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/yaml" || mediaType == "application/x-yaml" || mediaType == "text/yaml"
}

// WantsYAML reports whether the caller asked for YAML, with ?format=yaml or an Accept header naming a YAML type.
func WantsYAML(r *http.Request) bool {
	if f := r.URL.Query().Get("format"); f != "" {
		return strings.EqualFold(f, "yaml") || strings.EqualFold(f, "yml")
	}
	for _, part := range strings.Split(r.Header.Get("Accept"), ",") {
		if isYAMLType(strings.TrimSpace(part)) {
			return true
		}
	}
	return false
}

// DecodeDocument reads a JSON or YAML body, chosen by Content-Type, into v. Unknown fields are refused, because a document is often typed by hand and a misspelt key would otherwise be silently dropped.
func DecodeDocument(r *http.Request, v any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxDocumentBytes+1))
	if err != nil {
		return fmt.Errorf("%w: reading body: %v", ErrDocumentInvalid, err)
	}
	if len(body) > maxDocumentBytes {
		return fmt.Errorf("%w: body is larger than %d bytes", ErrDocumentInvalid, maxDocumentBytes)
	}
	if isYAMLType(r.Header.Get("Content-Type")) {
		if body, err = yaml.YAMLToJSON(body); err != nil {
			return fmt.Errorf("%w: %v", ErrDocumentInvalid, err)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %v", ErrDocumentInvalid, err)
	}
	if dec.More() {
		return fmt.Errorf("%w: unexpected content after the document", ErrDocumentInvalid)
	}
	return nil
}

// MarshalDocumentYAML encodes v as YAML through its JSON form, so the json tags decide the field names exactly as they do for JSON. Fields keep their declared order rather than being sorted, so a document reads in the order it is written: a binding's role before its members.
func MarshalDocumentYAML(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// JSON is YAML, and decoding into a node keeps key order where decoding into a map would not.
	var node yamlv3.Node
	if err := yamlv3.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	blockStyle(&node)
	var buf bytes.Buffer
	enc := yamlv3.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// blockStyle clears the flow style JSON input leaves on every node, so the output is ordinary block YAML.
func blockStyle(n *yamlv3.Node) {
	n.Style &^= yamlv3.FlowStyle
	if n.Kind == yamlv3.ScalarNode && n.Tag == "!!str" {
		// Quote only where YAML needs it, as a person would write it.
		n.Style &^= yamlv3.DoubleQuotedStyle
	}
	for _, c := range n.Content {
		blockStyle(c)
	}
}

// RespondDocument writes v as YAML when the caller asked for it (see WantsYAML) and as JSON otherwise.
func RespondDocument(w http.ResponseWriter, r *http.Request, status int, v any) {
	if !WantsYAML(r) {
		RespondJSON(w, status, v)
		return
	}
	out, err := MarshalDocumentYAML(v)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to encode document")
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(status)
	_, _ = w.Write(out)
}

// RespondDocumentError answers a document that could not be decoded or validated.
func RespondDocumentError(w http.ResponseWriter, status int, message string, fields []DocumentFieldError) {
	RespondJSON(w, status, DocumentErrorResponse{Error: message, Fields: fields})
}
