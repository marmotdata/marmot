package mcp

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/glossary"
)

// FormatAssetCard creates a detailed view of an asset
func FormatAssetCard(a *asset.Asset, marmotURL string) string {
	var parts []string

	// Header
	parts = append(parts, fmt.Sprintf("## %s", escapeMarkdown(*a.Name)))

	// Asset link
	if marmotURL != "" {
		parts = append(parts, fmt.Sprintf("[View in Marmot](%s/discover/%s/%s)", strings.TrimSuffix(marmotURL, "/"), a.Type, *a.Name))
	}

	// Type and provider
	parts = append(parts, fmt.Sprintf("**Type:** %s", a.Type))
	if len(a.Providers) > 0 {
		parts = append(parts, fmt.Sprintf("**Provider:** %s", strings.Join(a.Providers, ", ")))
	}

	// MRN if available
	if a.MRN != nil && *a.MRN != "" {
		parts = append(parts, fmt.Sprintf("**MRN:** `%s`", *a.MRN))
	}

	// Description
	if a.Description != nil && *a.Description != "" {
		parts = append(parts, "")
		parts = append(parts, *a.Description)
	}

	// Tags
	if len(a.Tags) > 0 {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("**Tags:** %s", strings.Join(a.Tags, ", ")))
	}

	// Schema (columns and types)
	if len(a.Schema) > 0 {
		parts = append(parts, "")
		parts = append(parts, "**Schema:**")
		for col, colType := range a.Schema {
			parts = append(parts, fmt.Sprintf("- `%s`: %s", col, colType))
		}
	}

	// Query definition
	if a.Query != nil && *a.Query != "" {
		lang := "sql"
		if a.QueryLanguage != nil && *a.QueryLanguage != "" {
			lang = *a.QueryLanguage
		}
		parts = append(parts, "")
		parts = append(parts, "**Query:**")
		parts = append(parts, fmt.Sprintf("```%s\n%s\n```", lang, *a.Query))
	}

	// Metadata
	if len(a.Metadata) > 0 {
		parts = append(parts, "")
		parts = append(parts, "**Metadata:**")
		for key, value := range a.Metadata {
			parts = append(parts, fmt.Sprintf("- `%s`: %v", key, value))
		}
	}

	return strings.Join(parts, "\n")
}

// FormatAssetList creates a compact list of assets
func FormatAssetList(assets []*asset.Asset, total int, marmotURL string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("# Found %d assets", total))

	if total > len(assets) {
		parts = append(parts, fmt.Sprintf("_Showing %d of %d results (use pagination for more)_", len(assets), total))
	}
	parts = append(parts, "")

	if len(assets) == 0 {
		parts = append(parts, "No assets found matching your criteria.")
		return strings.Join(parts, "\n")
	}

	// Group by type
	byType := make(map[string][]*asset.Asset)
	for _, a := range assets {
		byType[a.Type] = append(byType[a.Type], a)
	}

	// Show each type group
	for assetType, typeAssets := range byType {
		parts = append(parts, fmt.Sprintf("### %s (%d)", assetType, len(typeAssets)))
		parts = append(parts, "")

		for _, a := range typeAssets {
			name := *a.Name
			if marmotURL != "" {
				parts = append(parts, fmt.Sprintf("- [%s](%s/discover/%s/%s)", escapeMarkdown(name), strings.TrimSuffix(marmotURL, "/"), a.Type, name))
			} else {
				parts = append(parts, fmt.Sprintf("- %s", escapeMarkdown(name)))
			}

			// Add inline metadata for interesting properties
			var inline []string
			if a.MRN != nil && *a.MRN != "" {
				inline = append(inline, fmt.Sprintf("`%s`", *a.MRN))
			}
			if len(a.Providers) > 0 {
				inline = append(inline, a.Providers[0])
			}
			if len(inline) > 0 {
				parts[len(parts)-1] += " — " + strings.Join(inline, ", ")
			}

			// Add description if short
			if a.Description != nil && *a.Description != "" && len(*a.Description) < 100 {
				parts = append(parts, fmt.Sprintf("  _%s_", *a.Description))
			}
		}
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FormatOwnershipResult creates a rich ownership display
func FormatOwnershipResult(ownerName, ownerType string, assets []*asset.Asset, terms []*glossary.GlossaryTerm, marmotURL string) string {
	var parts []string

	// Header
	parts = append(parts, fmt.Sprintf("# Resources owned by %s (%s)", ownerName, ownerType))
	parts = append(parts, "")

	// Assets section
	if len(assets) > 0 {
		parts = append(parts, fmt.Sprintf("## Data Assets (%d)", len(assets)))
		parts = append(parts, "")

		// Group by type
		byType := make(map[string][]*asset.Asset)
		for _, a := range assets {
			byType[a.Type] = append(byType[a.Type], a)
		}

		for assetType, typeAssets := range byType {
			parts = append(parts, fmt.Sprintf("### %s (%d)", assetType, len(typeAssets)))
			for _, a := range typeAssets {
				name := *a.Name
				if marmotURL != "" {
					parts = append(parts, fmt.Sprintf("- [%s](%s/discover/%s/%s)", escapeMarkdown(name), strings.TrimSuffix(marmotURL, "/"), a.Type, name))
				} else {
					parts = append(parts, fmt.Sprintf("- %s", escapeMarkdown(name)))
				}

				if a.Description != nil && *a.Description != "" && len(*a.Description) < 80 {
					parts = append(parts, fmt.Sprintf("  _%s_", *a.Description))
				}
			}
			parts = append(parts, "")
		}
	} else {
		parts = append(parts, "## Data Assets")
		parts = append(parts, "_No data assets owned_")
		parts = append(parts, "")
	}

	// Glossary terms section
	if len(terms) > 0 {
		parts = append(parts, fmt.Sprintf("## Glossary Terms (%d)", len(terms)))
		parts = append(parts, "")

		for _, term := range terms {
			if marmotURL != "" {
				parts = append(parts, fmt.Sprintf("### [%s](%s/glossary/%s)", escapeMarkdown(term.Name), strings.TrimSuffix(marmotURL, "/"), term.ID))
			} else {
				parts = append(parts, fmt.Sprintf("### %s", escapeMarkdown(term.Name)))
			}
			parts = append(parts, term.Definition)
			parts = append(parts, "")
		}
	} else {
		parts = append(parts, "## Glossary Terms")
		parts = append(parts, "_No glossary terms owned_")
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FormatTermCard creates a rich glossary term display
func FormatTermCard(term *glossary.GlossaryTerm, marmotURL string) string {
	var parts []string

	// Header
	parts = append(parts, fmt.Sprintf("# %s", escapeMarkdown(term.Name)))
	parts = append(parts, "")

	// Link
	if marmotURL != "" {
		parts = append(parts, fmt.Sprintf("[View in Marmot](%s/glossary/%s)", strings.TrimSuffix(marmotURL, "/"), term.ID))
		parts = append(parts, "")
	}

	// Definition (highlighted)
	parts = append(parts, "## Definition")
	parts = append(parts, fmt.Sprintf("> %s", term.Definition))
	parts = append(parts, "")

	// Extended description
	if term.Description != nil && *term.Description != "" {
		parts = append(parts, "## Description")
		parts = append(parts, *term.Description)
		parts = append(parts, "")
	}

	// Owners
	if len(term.Owners) > 0 {
		parts = append(parts, "## Owners")
		for _, owner := range term.Owners {
			parts = append(parts, fmt.Sprintf("- %s (%s)", owner.Name, owner.Type))
		}
		parts = append(parts, "")
	}

	// Hierarchy
	if term.ParentTermID != nil && *term.ParentTermID != "" {
		parts = append(parts, fmt.Sprintf("**Parent Term:** `%s`", *term.ParentTermID))
		parts = append(parts, "")
	}

	// Tags
	if len(term.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("**Tags:** %s", strings.Join(term.Tags, ", ")))
		parts = append(parts, "")
	}

	// Metadata
	if len(term.Metadata) > 0 {
		parts = append(parts, "## Additional Properties")
		for key, value := range term.Metadata {
			parts = append(parts, fmt.Sprintf("- **%s**: %v", key, value))
		}
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FormatSearchSummary creates a summary with filters
func FormatSearchSummary(total, count int, filters interface{}) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("**Showing %d of %d total results**", count, total))

	if filters != nil {
		parts = append(parts, "")
		parts = append(parts, "**Available Filters:**")

		// Try to extract types/providers/tags regardless of the struct type
		switch f := filters.(type) {
		case map[string]interface{}:
			if types, ok := f["types"].(map[string]int); ok && len(types) > 0 {
				var typeList []string
				for t, c := range types {
					typeList = append(typeList, fmt.Sprintf("%s (%d)", t, c))
				}
				parts = append(parts, fmt.Sprintf("- Types: %s", strings.Join(typeList, ", ")))
			}

			if providers, ok := f["providers"].(map[string]int); ok && len(providers) > 0 {
				var providerList []string
				for p, c := range providers {
					providerList = append(providerList, fmt.Sprintf("%s (%d)", p, c))
				}
				parts = append(parts, fmt.Sprintf("- Providers: %s", strings.Join(providerList, ", ")))
			}

			if tags, ok := f["tags"].(map[string]int); ok && len(tags) > 0 {
				var tagList []string
				count := 0
				for t, c := range tags {
					if count >= 10 {
						tagList = append(tagList, "...")
						break
					}
					tagList = append(tagList, fmt.Sprintf("%s (%d)", t, c))
					count++
				}
				parts = append(parts, fmt.Sprintf("- Tags: %s", strings.Join(tagList, ", ")))
			}
		default:
			// If it's a struct, use reflection or just skip for now
			parts = append(parts, "_Filter details available in full query_")
		}
	}

	return strings.Join(parts, "\n")
}

// FormatNextActions formats suggested next actions
func FormatNextActions(actions map[string]string) string {
	if len(actions) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "---")
	parts = append(parts, "")
	parts = append(parts, "## Next steps")
	parts = append(parts, "")

	for label, action := range actions {
		parts = append(parts, fmt.Sprintf("**%s**", label))
		parts = append(parts, fmt.Sprintf("```json\n%s\n```", action))
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FormatDataProductCard creates a rich data product display including member assets.
func FormatDataProductCard(product *dataproduct.DataProduct, memberAssets []*asset.Asset, totalAssets int, marmotURL string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("# %s", escapeMarkdown(product.Name)))
	parts = append(parts, "")

	if marmotURL != "" {
		parts = append(parts, fmt.Sprintf("[View in Marmot](%s/products/%s)", strings.TrimSuffix(marmotURL, "/"), product.ID))
		parts = append(parts, "")
	}

	if product.Description != nil && *product.Description != "" {
		parts = append(parts, *product.Description)
		parts = append(parts, "")
	}

	if len(product.Owners) > 0 {
		parts = append(parts, "## Owners")
		for _, owner := range product.Owners {
			ownerLine := fmt.Sprintf("- **%s** (%s)", escapeMarkdown(owner.Name), owner.Type)
			if owner.Email != nil && *owner.Email != "" {
				ownerLine += fmt.Sprintf(" — %s", *owner.Email)
			}
			parts = append(parts, ownerLine)
		}
		parts = append(parts, "")
	}

	if len(product.Rules) > 0 {
		parts = append(parts, "## Membership Rules")
		for _, rule := range product.Rules {
			status := "enabled"
			if !rule.IsEnabled {
				status = "disabled"
			}
			ruleLine := fmt.Sprintf("- **%s** (%s, %s)", escapeMarkdown(rule.Name), rule.RuleType, status)
			if rule.MatchedAssetCount > 0 {
				ruleLine += fmt.Sprintf(" — %d matched assets", rule.MatchedAssetCount)
			}
			parts = append(parts, ruleLine)
		}
		parts = append(parts, "")
	}

	parts = append(parts, fmt.Sprintf("## Member Assets (%d)", totalAssets))
	parts = append(parts, "")
	if len(memberAssets) == 0 {
		parts = append(parts, "_No assets in this data product_")
	} else {
		for _, a := range memberAssets {
			name := *a.Name
			if marmotURL != "" {
				parts = append(parts, fmt.Sprintf("- [%s](%s/discover/%s/%s) (%s)", escapeMarkdown(name), strings.TrimSuffix(marmotURL, "/"), a.Type, name, a.Type))
			} else {
				parts = append(parts, fmt.Sprintf("- %s (%s)", escapeMarkdown(name), a.Type))
			}
		}
		if totalAssets > len(memberAssets) {
			parts = append(parts, fmt.Sprintf("_...and %d more assets_", totalAssets-len(memberAssets)))
		}
	}
	parts = append(parts, "")

	if len(product.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("**Tags:** %s", strings.Join(product.Tags, ", ")))
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FormatDataProductList creates a compact list of data products.
func FormatDataProductList(products []*dataproduct.DataProduct, total int, marmotURL string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("# Found %d data products", total))

	if total > len(products) {
		parts = append(parts, fmt.Sprintf("_Showing %d of %d results (use pagination for more)_", len(products), total))
	}
	parts = append(parts, "")

	if len(products) == 0 {
		parts = append(parts, "No data products found matching your criteria.")
		return strings.Join(parts, "\n")
	}

	for _, product := range products {
		if marmotURL != "" {
			parts = append(parts, fmt.Sprintf("- [%s](%s/products/%s) — %d assets", escapeMarkdown(product.Name), strings.TrimSuffix(marmotURL, "/"), product.ID, product.AssetCount))
		} else {
			parts = append(parts, fmt.Sprintf("- %s — %d assets", escapeMarkdown(product.Name), product.AssetCount))
		}

		if product.Description != nil && *product.Description != "" && len(*product.Description) < 100 {
			parts = append(parts, fmt.Sprintf("  _%s_", *product.Description))
		}
	}

	return strings.Join(parts, "\n")
}

// FormatAssetDataProducts formats the data products an asset belongs to.
func FormatAssetDataProducts(products []*dataproduct.DataProduct, marmotURL string) string {
	var parts []string
	parts = append(parts, "## Data Products")
	parts = append(parts, "")

	for _, product := range products {
		if marmotURL != "" {
			parts = append(parts, fmt.Sprintf("- [%s](%s/products/%s)", escapeMarkdown(product.Name), strings.TrimSuffix(marmotURL, "/"), product.ID))
		} else {
			parts = append(parts, fmt.Sprintf("- %s", escapeMarkdown(product.Name)))
		}
	}

	return strings.Join(parts, "\n")
}

// FormatTeamCard creates a rich team display including members.
func FormatTeamCard(team *Team, members []*TeamMember, marmotURL string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("# %s", escapeMarkdown(team.Name)))
	parts = append(parts, "")

	if marmotURL != "" {
		parts = append(parts, fmt.Sprintf("[View in Marmot](%s/teams/%s)", strings.TrimSuffix(marmotURL, "/"), team.ID))
		parts = append(parts, "")
	}

	if team.Description != "" {
		parts = append(parts, team.Description)
		parts = append(parts, "")
	}

	parts = append(parts, fmt.Sprintf("## Members (%d)", len(members)))
	parts = append(parts, "")
	if len(members) == 0 {
		parts = append(parts, "_No members in this team_")
	} else {
		for _, member := range members {
			memberLine := fmt.Sprintf("- **%s** (@%s) — %s", escapeMarkdown(member.Name), member.Username, member.Role)
			if member.Email != nil && *member.Email != "" {
				memberLine += fmt.Sprintf(", %s", *member.Email)
			}
			parts = append(parts, memberLine)
		}
	}
	parts = append(parts, "")

	if len(team.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("**Tags:** %s", strings.Join(team.Tags, ", ")))
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FormatTeamList creates a compact list of teams.
func FormatTeamList(teams []*Team, total int, marmotURL string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("# Found %d teams", total))

	if total > len(teams) {
		parts = append(parts, fmt.Sprintf("_Showing %d of %d results (use pagination for more)_", len(teams), total))
	}
	parts = append(parts, "")

	if len(teams) == 0 {
		parts = append(parts, "No teams found.")
		return strings.Join(parts, "\n")
	}

	for _, team := range teams {
		parts = append(parts, formatTeamListEntry(team, marmotURL))
	}

	return strings.Join(parts, "\n")
}

// formatTeamListEntry formats a single team as a list item.
func formatTeamListEntry(team *Team, marmotURL string) string {
	var entry string
	if marmotURL != "" {
		entry = fmt.Sprintf("- [%s](%s/teams/%s)", escapeMarkdown(team.Name), strings.TrimSuffix(marmotURL, "/"), team.ID)
	} else {
		entry = fmt.Sprintf("- %s", escapeMarkdown(team.Name))
	}

	if team.Description != "" && len(team.Description) < 100 {
		entry += fmt.Sprintf("\n  _%s_", team.Description)
	}

	return entry
}

// FormatTermList creates a compact list of glossary terms.
func FormatTermList(terms []*glossary.GlossaryTerm, total int, marmotURL string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("# Found %d glossary terms", total))

	if total > len(terms) {
		parts = append(parts, fmt.Sprintf("_Showing %d of %d results (use pagination for more)_", len(terms), total))
	}
	parts = append(parts, "")

	if len(terms) == 0 {
		parts = append(parts, "No glossary terms found matching your criteria.")
		return strings.Join(parts, "\n")
	}

	for _, term := range terms {
		if marmotURL != "" {
			parts = append(parts, fmt.Sprintf("- [%s](%s/glossary/%s)", escapeMarkdown(term.Name), strings.TrimSuffix(marmotURL, "/"), term.ID))
		} else {
			parts = append(parts, fmt.Sprintf("- %s", escapeMarkdown(term.Name)))
		}

		definition := term.Definition
		if len(definition) > 120 {
			definition = definition[:117] + "..."
		}
		if definition != "" {
			parts = append(parts, fmt.Sprintf("  _%s_", definition))
		}
	}

	return strings.Join(parts, "\n")
}

// FormatTermHierarchy formats a set of related terms (ancestors or children) under a heading.
func FormatTermHierarchy(heading string, terms []*glossary.GlossaryTerm) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("## %s", heading))
	parts = append(parts, "")

	for _, term := range terms {
		parts = append(parts, fmt.Sprintf("- **%s** (`%s`)", escapeMarkdown(term.Name), term.ID))
	}

	return strings.Join(parts, "\n")
}

// FormatTermAssets formats the assets linked to a glossary term.
func FormatTermAssets(assets []*asset.Asset, total int, marmotURL string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("## Linked Assets (%d)", total))
	parts = append(parts, "")

	for _, a := range assets {
		name := *a.Name
		if marmotURL != "" {
			parts = append(parts, fmt.Sprintf("- [%s](%s/discover/%s/%s) (%s)", escapeMarkdown(name), strings.TrimSuffix(marmotURL, "/"), a.Type, name, a.Type))
		} else {
			parts = append(parts, fmt.Sprintf("- %s (%s)", escapeMarkdown(name), a.Type))
		}
	}

	if total > len(assets) {
		parts = append(parts, fmt.Sprintf("_...and %d more assets_", total-len(assets)))
	}

	return strings.Join(parts, "\n")
}

// escapeMarkdown escapes markdown special characters in a string.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
	)
	return replacer.Replace(s)
}
