package mcp

import (
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/glossary"
)

// FormatAssetCard creates a visually appealing card for an asset
func FormatAssetCard(a *asset.Asset, marmotURL string) string {
	var parts []string

	// Header with icon
	icon := getAssetIcon(a.Type)
	parts = append(parts, fmt.Sprintf("## %s %s", icon, escapeMarkdown(*a.Name)))

	// Asset link
	if marmotURL != "" {
		parts = append(parts, fmt.Sprintf("🔗 [View in Marmot](%s/discover/%s/%s)", strings.TrimSuffix(marmotURL, "/"), a.Type, *a.Name))
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
		parts = append(parts, fmt.Sprintf("📝 %s", *a.Description))
	}

	// Tags
	if len(a.Tags) > 0 {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("🏷️  %s", strings.Join(a.Tags, " · ")))
	}

	// Key metadata
	if a.Metadata != nil && len(a.Metadata) > 0 {
		parts = append(parts, "")
		parts = append(parts, "**Key Properties:**")

		// Show interesting metadata (limit to 5 most relevant)
		count := 0
		for key, value := range a.Metadata {
			if count >= 5 {
				break
			}
			parts = append(parts, fmt.Sprintf("- `%s`: %v", key, value))
			count++
		}
	}

	return strings.Join(parts, "\n")
}

// FormatAssetList creates a compact list of assets
func FormatAssetList(assets []*asset.Asset, total int, marmotURL string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("# 📊 Found %d assets", total))

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
		icon := getAssetIcon(assetType)
		parts = append(parts, fmt.Sprintf("### %s %s (%d)", icon, assetType, len(typeAssets)))
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
				parts[len(parts)-1] += " · " + strings.Join(inline, " · ")
			}

			// Add description if short
			if a.Description != nil && *a.Description != "" && len(*a.Description) < 100 {
				parts = append(parts, fmt.Sprintf("  _↳ %s_", *a.Description))
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
	icon := "👤"
	if ownerType == "team" {
		icon = "👥"
	}
	parts = append(parts, fmt.Sprintf("# %s %s's Data Catalog", icon, ownerName))
	parts = append(parts, "")

	// Assets section
	if len(assets) > 0 {
		parts = append(parts, fmt.Sprintf("## 📦 Data Assets (%d)", len(assets)))
		parts = append(parts, "")

		// Group by type
		byType := make(map[string][]*asset.Asset)
		for _, a := range assets {
			byType[a.Type] = append(byType[a.Type], a)
		}

		for assetType, typeAssets := range byType {
			icon := getAssetIcon(assetType)
			parts = append(parts, fmt.Sprintf("### %s %s (%d)", icon, assetType, len(typeAssets)))
			for _, a := range typeAssets {
				name := *a.Name
				if marmotURL != "" {
					parts = append(parts, fmt.Sprintf("- [%s](%s/discover/%s/%s)", escapeMarkdown(name), strings.TrimSuffix(marmotURL, "/"), a.Type, name))
				} else {
					parts = append(parts, fmt.Sprintf("- %s", escapeMarkdown(name)))
				}

				if a.Description != nil && *a.Description != "" && len(*a.Description) < 80 {
					parts = append(parts, fmt.Sprintf("  _↳ %s_", *a.Description))
				}
			}
			parts = append(parts, "")
		}
	} else {
		parts = append(parts, "## 📦 Data Assets")
		parts = append(parts, "_No data assets owned_")
		parts = append(parts, "")
	}

	// Glossary terms section
	if len(terms) > 0 {
		parts = append(parts, fmt.Sprintf("## 📚 Glossary Terms (%d)", len(terms)))
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
		parts = append(parts, "## 📚 Glossary Terms")
		parts = append(parts, "_No glossary terms owned_")
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FormatTermCard creates a rich glossary term display
func FormatTermCard(term *glossary.GlossaryTerm, marmotURL string) string {
	var parts []string

	// Header
	parts = append(parts, fmt.Sprintf("# 📖 %s", escapeMarkdown(term.Name)))
	parts = append(parts, "")

	// Link
	if marmotURL != "" {
		parts = append(parts, fmt.Sprintf("🔗 [View in Marmot](%s/glossary/%s)", strings.TrimSuffix(marmotURL, "/"), term.ID))
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
		parts = append(parts, "## 👥 Owners")
		for _, owner := range term.Owners {
			icon := "👤"
			if owner.Type == "team" {
				icon = "👥"
			}
			parts = append(parts, fmt.Sprintf("- %s %s", icon, owner.Name))
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
		parts = append(parts, fmt.Sprintf("🏷️  %s", strings.Join(term.Tags, " · ")))
		parts = append(parts, "")
	}

	// Metadata
	if term.Metadata != nil && len(term.Metadata) > 0 {
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
	parts = append(parts, "## 💡 What's Next?")
	parts = append(parts, "")

	for label, action := range actions {
		parts = append(parts, fmt.Sprintf("**%s**", label))
		parts = append(parts, fmt.Sprintf("```json\n%s\n```", action))
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// Helper: Get icon for asset type
func getAssetIcon(assetType string) string {
	icons := map[string]string{
		"table":     "📊",
		"view":      "👁️",
		"topic":     "📨",
		"queue":     "📬",
		"bucket":    "🪣",
		"database":  "💾",
		"schema":    "📐",
		"dashboard": "📈",
		"api":       "🔌",
		"file":      "📄",
		"stream":    "🌊",
	}

	if icon, ok := icons[strings.ToLower(assetType)]; ok {
		return icon
	}
	return "📦" // default
}

// Helper: Escape markdown special characters
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
