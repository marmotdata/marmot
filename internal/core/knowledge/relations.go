package knowledge

// Relation is how an entry relates to another asset, read from either end: a policy applies to a table, and seen from the table it applies here.
type Relation struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	InverseLabel string `json:"inverse_label"`
	Description  string `json:"description"`
} // @name KnowledgeRelation

// RelationAppliesTo is the default relation, used when a caller names none.
const RelationAppliesTo = "applies_to"

var relations = []Relation{
	{ID: RelationAppliesTo, Label: "Applies to", InverseLabel: "Applies here", Description: "The entry governs it: a policy for a table, a runbook for a service, a design system for an app."},
	{ID: "documents", Label: "Documents", InverseLabel: "Documented by", Description: "The entry explains it, such as a decision record for a pipeline it shaped."},
	{ID: "related_to", Label: "Related to", InverseLabel: "Related", Description: "Worth reading alongside it, without governing or explaining it."},
}

// Relations returns every relation, in the order they are offered.
func Relations() []Relation {
	return relations
}

func relationByID(id string) (Relation, bool) {
	for _, r := range relations {
		if r.ID == id {
			return r, true
		}
	}
	return Relation{}, false
}

// Selector types a rule can match an asset by. Tags match as written; types and providers ignore case.
const (
	SelectorTag      = "tag"
	SelectorType     = "type"
	SelectorProvider = "provider"
)

// Where an entry about an asset came from: linked to the asset itself, matched by a rule, or linked to something that contains it.
const (
	SourceDirect    = "direct"
	SourceRule      = "rule"
	SourceInherited = "inherited"
)

// stateRanks orders states the way lists show them, and matches knowledge_state_rank in migration 88.
var stateRanks = map[State]int16{StateCurrent: 0, StatePending: 1, StateHistorical: 2}

// rankOf is the rank of a status, matching knowledge_state_rank in SQL.
func rankOf(status string) int16 {
	for _, k := range kinds {
		if st, ok := k.status(status); ok {
			return stateRanks[st.State]
		}
	}
	return 3
}
