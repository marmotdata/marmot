package sync

import (
	"fmt"
	"sort"
)

type ChangeType string

const (
	Created  ChangeType = "created"
	Updated  ChangeType = "updated"
	NoChange ChangeType = "no_change"
)

// Summary tracks the changes made during a sync operation
type Summary struct {
	Assets        map[string]ChangeType
	Lineage       map[string]ChangeType
	Documentation map[string]ChangeType
}

// Print outputs a formatted summary of all changes
func (s *Summary) Print() {
	fmt.Println("\nOperation complete! Summary of changes:")

	fmt.Println("\nAssets:")
	fmt.Println("-------")
	s.printSection(s.Assets)

	fmt.Println("\nLineage:")
	fmt.Println("--------")
	s.printSection(s.Lineage)

	fmt.Println("\nDocumentation:")
	fmt.Println("--------------")
	s.printSection(s.Documentation)

	// Print totals
	fmt.Println("\nTotals:")
	fmt.Println("-------")
	fmt.Printf("Assets: %d total (%d created, %d updated, %d unchanged)\n",
		len(s.Assets),
		s.CountByType(s.Assets, Created),
		s.CountByType(s.Assets, Updated),
		s.CountByType(s.Assets, NoChange))
	fmt.Printf("Lineage: %d total (%d created, %d updated, %d unchanged)\n",
		len(s.Lineage),
		s.CountByType(s.Lineage, Created),
		s.CountByType(s.Lineage, Updated),
		s.CountByType(s.Lineage, NoChange))
	fmt.Printf("Documentation: %d total (%d created, %d updated, %d unchanged)\n",
		len(s.Documentation),
		s.CountByType(s.Documentation, Created),
		s.CountByType(s.Documentation, Updated),
		s.CountByType(s.Documentation, NoChange))
}

// printSection outputs a formatted section of changes
func (s *Summary) printSection(items map[string]ChangeType) {
	if len(items) == 0 {
		fmt.Println("  No changes")
		return
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		var symbol string
		switch items[k] {
		case Created:
			symbol = "+"
		case Updated:
			symbol = "~"
		case NoChange:
			symbol = " "
		}
		fmt.Printf("  %s %s\n", symbol, k)
	}
}

// CountByType counts the number of items with a specific change type
func (s *Summary) CountByType(items map[string]ChangeType, changeType ChangeType) int {
	count := 0
	for _, t := range items {
		if t == changeType {
			count++
		}
	}
	return count
}
