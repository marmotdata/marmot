package search

import (
	"testing"

	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/stretchr/testify/assert"
)

func TestMatchedCount(t *testing.T) {
	results := []*search.Result{{ID: "a"}, {ID: "b"}}

	tests := []struct {
		name     string
		response *search.Response
		want     int
	}{
		{name: "text search without a total counts its results", response: &search.Response{Results: results}, want: 2},
		{name: "later page counts the pages before it", response: &search.Response{Results: results, Offset: 20}, want: 22},
		{name: "total wins when it is reported", response: &search.Response{Results: results, Total: 57}, want: 57},
		{name: "search that found nothing is empty", response: &search.Response{Results: []*search.Result{}}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchedCount(tt.response))
		})
	}
}
