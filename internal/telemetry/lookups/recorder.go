package lookups

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// Category identifies what kind of lookup happened.
type Category string

const (
	CategoryAssetDetail  Category = "asset_detail"
	CategoryLineage      Category = "lineage"
	CategoryGlossaryTerm Category = "glossary_term"
	CategoryDataProduct  Category = "data_product"
)

// Source identifies the channel a lookup came in on. Values are stable —
// they end up in the telemetry payload.
const (
	SourceHTTP   = "http"
	SourceCLI    = "cli"
	SourceSDKGo  = "sdk-go"
	SourceSDKTS  = "sdk-ts"
	SourceSDKPy  = "sdk-py"
	SourceWeb    = "web"
	SourceMCP    = "mcp"
	SourceOther  = "other"
)

type sourceCtxKey struct{}

// WithSource attaches a source label to ctx. Downstream Record calls read it.
func WithSource(ctx context.Context, source string) context.Context {
	if source == "" {
		return ctx
	}
	return context.WithValue(ctx, sourceCtxKey{}, source)
}

// SourceFrom returns the source label attached to ctx, or SourceOther.
func SourceFrom(ctx context.Context) string {
	if v, ok := ctx.Value(sourceCtxKey{}).(string); ok && v != "" {
		return v
	}
	return SourceOther
}

// SourceFromRequest canonicalises client identity into one of the Source*
// constants. Precedence: X-Marmot-Client header, then User-Agent prefix
// matching. Browsers can't reliably set UA so the web app sets the header.
func SourceFromRequest(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Marmot-Client")); v != "" {
		switch strings.ToLower(v) {
		case "web":
			return SourceWeb
		case "cli":
			return SourceCLI
		case "sdk-go", "go":
			return SourceSDKGo
		case "sdk-ts", "ts":
			return SourceSDKTS
		case "sdk-py", "py", "python":
			return SourceSDKPy
		case "mcp":
			return SourceMCP
		}
	}
	ua := r.Header.Get("User-Agent")
	switch {
	case ua == "":
		return SourceHTTP
	case strings.HasPrefix(ua, "marmot-cli"):
		return SourceCLI
	case strings.HasPrefix(ua, "marmot-sdk-go"):
		return SourceSDKGo
	case strings.HasPrefix(ua, "marmot-sdk-ts"):
		return SourceSDKTS
	case strings.HasPrefix(ua, "marmot-sdk-py"):
		return SourceSDKPy
	case strings.HasPrefix(ua, "marmot-web"):
		return SourceWeb
	}
	return SourceHTTP
}

// Recorder counts lookups. Record is safe for concurrent use and does no I/O.
type Recorder interface {
	Record(ctx context.Context, category Category)
	Snapshot() Snapshot
}

// Snapshot is source -> category -> count, at a point in time.
type Snapshot map[string]map[string]int64

type key struct {
	source   string
	category Category
}

type inMemoryRecorder struct {
	counters sync.Map // key -> *atomic.Int64
}

// NewRecorder returns an in-memory Recorder. Nothing to close.
func NewRecorder() Recorder {
	return &inMemoryRecorder{}
}

func (r *inMemoryRecorder) Record(ctx context.Context, category Category) {
	k := key{source: SourceFrom(ctx), category: category}
	if v, ok := r.counters.Load(k); ok {
		v.(*atomic.Int64).Add(1)
		return
	}
	var fresh atomic.Int64
	fresh.Add(1)
	actual, loaded := r.counters.LoadOrStore(k, &fresh)
	if loaded {
		actual.(*atomic.Int64).Add(1)
	}
}

// Snapshot atomically drains the current in-memory counters. Once returned,
// those counts are gone from memory — the caller is responsible for persisting
// them (or accepting the loss on shutdown).
func (r *inMemoryRecorder) Snapshot() Snapshot {
	out := Snapshot{}
	r.counters.Range(func(k, v any) bool {
		kk := k.(key)
		n := v.(*atomic.Int64).Swap(0)
		if n == 0 {
			return true
		}
		if _, ok := out[kk.source]; !ok {
			out[kk.source] = map[string]int64{}
		}
		out[kk.source][string(kk.category)] += n
		return true
	})
	return out
}
