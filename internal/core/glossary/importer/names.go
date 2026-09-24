package importer

import (
	"strings"

	"github.com/marmotdata/marmot/internal/core/glossary"
)

func fold(name string) string { return strings.ToLower(name) }

// names finds terms and file rows by name: an exact match first, then one
// that differs only in case, so a file typed by hand still finds its terms.
// Marmot does not make term names unique, so a lookup may return several.
type names struct {
	exact  map[string][]*glossary.GlossaryTerm
	folded map[string][]*glossary.GlossaryTerm
	// written maps a folded name to the rows' names as written, and lines to
	// their line numbers.
	written map[string][]string
	lines   map[string][]int
}

func newNames(found map[string][]*glossary.GlossaryTerm) names {
	n := names{exact: map[string][]*glossary.GlossaryTerm{}, folded: map[string][]*glossary.GlossaryTerm{}, written: map[string][]string{}, lines: map[string][]int{}}
	for _, terms := range found {
		for _, t := range terms {
			n.exact[t.Name] = append(n.exact[t.Name], t)
			n.folded[fold(t.Name)] = append(n.folded[fold(t.Name)], t)
		}
	}
	return n
}

func (n names) addRow(name string, line int) {
	n.written[fold(name)] = append(n.written[fold(name)], name)
	n.lines[fold(name)] = append(n.lines[fold(name)], line)
}

// term returns the catalog terms a name refers to, and whether they were
// found only by ignoring case.
func (n names) term(name string) ([]*glossary.GlossaryTerm, bool) {
	if m := n.exact[name]; len(m) > 0 {
		return m, false
	}
	m := n.folded[fold(name)]
	return m, len(m) > 0
}

func (n names) inFile(name string) bool { return len(n.lines[fold(name)]) > 0 }

// canonical is the name the catalog will use once the import is written: the
// existing term's, or the row's as written in the file.
func (n names) canonical(name string) string {
	if m, _ := n.term(name); len(m) == 1 {
		return m[0].Name
	}
	if w := n.written[fold(name)]; len(w) > 0 {
		return w[0]
	}
	return name
}
