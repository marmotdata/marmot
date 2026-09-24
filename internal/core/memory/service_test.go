package memory

import (
	"errors"
	"strings"
	"testing"
)

func TestContentIsOneShortLine(t *testing.T) {
	got, err := validateContent("  grain is\n\none row   per order line \t")
	if err != nil || got != "grain is one row per order line" {
		t.Errorf("whitespace should collapse to one line: %q, %v", got, err)
	}
	if _, err := validateContent(" \n\t "); !errors.Is(err, ErrInvalid) {
		t.Errorf("blank content: %v", err)
	}
	if _, err := validateContent(strings.Repeat("é", MaxContentLength)); err != nil {
		t.Errorf("content at the limit, counted in characters: %v", err)
	}
	if _, err := validateContent(strings.Repeat("x", MaxContentLength+1)); !errors.Is(err, ErrInvalid) {
		t.Errorf("content over the limit: %v", err)
	}
}
