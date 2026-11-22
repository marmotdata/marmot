package knowledge

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalid is wrapped by every error that a caller can fix by changing what they sent.
var ErrInvalid = errors.New("invalid knowledge entry")

const (
	maxText     = 500
	maxLongText = 20000
	maxListLen  = 100
	maxRows     = 200
	maxLinks    = 100
	dateLayout  = "2006-01-02"
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// cleanFields checks fields against the kind and returns them tidied: strings trimmed, empty values, list items and table rows dropped, so what is stored is only what someone wrote.
func cleanFields(k Kind, status string, in map[string]any) (map[string]any, error) {
	known := make(map[string]Field, len(k.Fields))
	for _, f := range k.Fields {
		known[f.Key] = f
	}
	for key := range in {
		if _, ok := known[key]; !ok {
			return nil, invalid("%s has no field %q", k.Label, key)
		}
	}
	out := make(map[string]any, len(in))
	for _, f := range k.Fields {
		v, err := cleanValue(f, in[f.Key])
		if err != nil {
			return nil, err
		}
		if v == nil {
			if f.Required {
				return nil, invalid("%s is required", f.Label)
			}
			if slices.Contains(f.RequiredWhen, status) {
				st, _ := k.status(status)
				return nil, invalid("%s is required once a %s is %s", f.Label, strings.ToLower(k.Label), strings.ToLower(st.Label))
			}
			continue
		}
		out[f.Key] = v
	}
	return out, nil
}

// cleanValue returns nil for an empty value.
func cleanValue(f Field, v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch f.Type {
	case FieldText, FieldLongText, FieldDate, FieldSelect, FieldURL:
		s, ok := v.(string)
		if !ok {
			return nil, invalid("%s must be text", f.Label)
		}
		return cleanScalar(f, s)
	case FieldList:
		items, ok := v.([]any)
		if !ok {
			return nil, invalid("%s must be a list of text", f.Label)
		}
		if len(items) > maxListLen {
			return nil, invalid("%s can hold at most %d items", f.Label, maxListLen)
		}
		var out []any
		for _, item := range items {
			s, ok := item.(string)
			if !ok {
				return nil, invalid("%s must be a list of text", f.Label)
			}
			if s = strings.TrimSpace(s); s == "" {
				continue
			}
			if utf8.RuneCountInString(s) > maxText {
				return nil, invalid("each item of %s must be at most %d characters", f.Label, maxText)
			}
			out = append(out, s)
		}
		if len(out) == 0 {
			return nil, nil
		}
		return out, nil
	case FieldTable:
		rows, ok := v.([]any)
		if !ok {
			return nil, invalid("%s must be a list of rows", f.Label)
		}
		limit := maxRows
		if f.MaxRows > 0 {
			limit = f.MaxRows
		}
		if len(rows) > limit {
			return nil, invalid("%s can hold at most %d rows", f.Label, limit)
		}
		var out []any
		for _, r := range rows {
			row, ok := r.(map[string]any)
			if !ok {
				return nil, invalid("each row of %s must be an object", f.Label)
			}
			cleaned := make(map[string]any, len(f.Columns))
			for key := range row {
				if !hasColumn(f, key) {
					return nil, invalid("%s has no column %q", f.Label, key)
				}
			}
			for _, c := range f.Columns {
				cell, ok := row[c.Key].(string)
				if row[c.Key] != nil && !ok {
					return nil, invalid("%s in %s must be text", c.Label, f.Label)
				}
				s, err := cleanScalar(c, cell)
				if err != nil {
					return nil, err
				}
				if s != nil {
					cleaned[c.Key] = s
				}
			}
			if len(cleaned) > 0 {
				out = append(out, cleaned)
			}
		}
		if len(out) == 0 {
			return nil, nil
		}
		return out, nil
	}
	return nil, invalid("%s has an unknown type", f.Label)
}

func hasColumn(f Field, key string) bool {
	for _, c := range f.Columns {
		if c.Key == key {
			return true
		}
	}
	return false
}

func cleanScalar(f Field, s string) (any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	limit := maxText
	if f.Type == FieldLongText {
		limit = maxLongText
	}
	if utf8.RuneCountInString(s) > limit {
		return nil, invalid("%s must be at most %d characters", f.Label, limit)
	}
	switch f.Type {
	case FieldDate:
		if _, err := time.Parse(dateLayout, s); err != nil {
			return nil, invalid("%s must be a date written as YYYY-MM-DD", f.Label)
		}
	case FieldURL:
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, invalid("%s must be a link starting with http:// or https://", f.Label)
		}
	case FieldSelect:
		ok := false
		for _, o := range f.Options {
			if o.Value == s {
				ok = true
				break
			}
		}
		if !ok {
			return nil, invalid("%s must be one of its listed options", f.Label)
		}
	}
	return s, nil
}

func cleanDate(label string, s *string) (*string, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	v := strings.TrimSpace(*s)
	if _, err := time.Parse(dateLayout, v); err != nil {
		return nil, invalid("%s must be a date written as YYYY-MM-DD", label)
	}
	return &v, nil
}

// CheckFields validates an entry's fields for a kind and status without saving, so a caller can show what would be stored, or what is wrong, before writing anything.
func CheckFields(k Kind, status string, in map[string]any) (map[string]any, error) {
	return cleanFields(k, status, in)
}

// DraftStatus is the status a new entry starts in when a person still has to agree it: the kind's first status that is not yet in force.
func (k Kind) DraftStatus() string {
	for _, s := range k.Statuses {
		if s.State == StatePending {
			return s.Value
		}
	}
	return k.DefaultStatus
}
