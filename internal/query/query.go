package query

// Query represents a parsed search query with specific conditions
type Query struct {
	FreeText string        // General text search
	Filters  []Filter      // Specific field filters
	Bool     *BooleanQuery // Boolean combinations of filters
}

// BooleanQuery represents a boolean combination of filters
type BooleanQuery struct {
	Must    []Filter      // AND conditions
	Should  []Filter      // OR conditions
	MustNot []Filter      // NOT conditions
	Nested  *BooleanQuery // Nested boolean query (for parentheses)
}

// Filter represents a single filter condition
type Filter struct {
	Field     []string    // Field to filter on (e.g., "metadata.owner")
	Operator  Operator    // Operator (e.g., =, :, contains)
	Value     interface{} // Value to compare against
	Range     *RangeValue // Range values for range queries
	OrigQuery string
}

// RangeValue represents a range query with optional bounds
type RangeValue struct {
	From      interface{}
	To        interface{}
	Inclusive bool
}

// Operator represents the type of filter operation
type Operator string

const (
	OpEquals       Operator = "="
	OpContains     Operator = "contains"
	OpNotEquals    Operator = "!="
	OpGreater      Operator = ">"
	OpLess         Operator = "<"
	OpGreaterEqual Operator = ">="
	OpLessEqual    Operator = "<="
	OpIn           Operator = "in"
	OpNotIn        Operator = "not in"
	OpRange        Operator = "range"
	OpWildcard     Operator = "wildcard"
	OpFreeText     Operator = "freetext"
)
