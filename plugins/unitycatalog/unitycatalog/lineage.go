package unitycatalog

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// tableReference matches the object named right after FROM or JOIN: up
// to three dot-separated parts, each optionally wrapped in backticks or
// double quotes. Subqueries start with "(" and so do not match.
var tableReference = regexp.MustCompile("(?i)\\b(?:from|join)\\s+((?:[`\"]?[\\w-]+[`\"]?)(?:\\.[`\"]?[\\w-]+[`\"]?){0,2})")

// buildReferenceLineage runs after every table is known and adds the
// edges that point from one table to another: a view to the tables it
// reads, and a table to the tables its foreign keys reference.
func (d *discovery) buildReferenceLineage() {
	for _, table := range d.tables {
		name := fullName(table.CatalogName, table.SchemaName, table.Name)
		ownMRN := d.tableMRNs[strings.ToLower(name)]

		switch table.TableType {
		case "VIEW", "MATERIALIZED_VIEW":
			for _, ref := range viewReferences(table) {
				resolved := resolveReference(ref, table.CatalogName, table.SchemaName)
				baseMRN, ok := d.tableMRNs[strings.ToLower(resolved)]
				if !ok {
					log.Debug().Str("view", name).Str("reference", resolved).
						Msg("Skipping VIEW_OF edge, referenced table not discovered")
					continue
				}
				d.addEdge(baseMRN, ownMRN, "VIEW_OF")
			}
		}

		for _, tc := range table.TableConstraints {
			if tc.ForeignKey == nil || tc.ForeignKey.ParentTable == "" {
				continue
			}
			resolved := resolveReference(tc.ForeignKey.ParentTable, table.CatalogName, table.SchemaName)
			parentMRN, ok := d.tableMRNs[strings.ToLower(resolved)]
			if !ok {
				log.Debug().Str("table", name).Str("parent", resolved).
					Msg("Skipping FOREIGN_KEY edge, parent table not discovered")
				continue
			}
			d.addEdge(ownMRN, parentMRN, "FOREIGN_KEY")
		}
	}
}

// viewReferences lists the tables a view reads: the dependencies the
// server recorded when it has them, plus every FROM and JOIN target in the
// SQL. The SQL scan is a best-effort read of the text, not a parser: it
// sees plain table references and misses comma-separated FROM lists.
func viewReferences(table tableInfo) []string {
	var refs []string

	if table.ViewDependencies != nil {
		for _, dep := range table.ViewDependencies.Dependencies {
			if dep.Table != nil && dep.Table.TableFullName != "" {
				refs = append(refs, dep.Table.TableFullName)
			}
		}
	}

	for _, match := range tableReference.FindAllStringSubmatch(table.ViewDefinition, -1) {
		refs = append(refs, strings.NewReplacer("`", "", `"`, "").Replace(match[1]))
	}

	return refs
}

// resolveReference expands a one- or two-part name to three parts using
// the catalog and schema of the object that mentions it, the way Unity
// Catalog resolves an unqualified name.
func resolveReference(ref, catalog, schema string) string {
	parts := strings.Split(ref, ".")
	switch len(parts) {
	case 1:
		return fullName(catalog, schema, parts[0])
	case 2:
		return fullName(catalog, parts[0], parts[1])
	default:
		return ref
	}
}

// addStorageEdge links an object to the cloud storage that holds it. The
// bucket or container asset itself belongs to the S3, GCS or Azure Blob
// plugin, so only the edge is emitted, using the exact identity that
// plugin produces; the server drops it when no such asset exists.
func (d *discovery) addStorageEdge(storageLocation, targetMRN string) {
	if storageMRN := storageContainerMRN(storageLocation); storageMRN != "" {
		d.addEdge(storageMRN, targetMRN, "FEEDS")
	}
}

// storageContainerMRN returns the MRN of the bucket or container a
// storage location lives in, or "" for locations that are not in cloud
// object storage (local files, DBFS, unknown schemes).
func storageContainerMRN(storageLocation string) string {
	u, err := url.Parse(storageLocation)
	if err != nil || u.Host == "" {
		return ""
	}

	switch strings.ToLower(u.Scheme) {
	case "s3", "s3a", "s3n":
		return mrn.New("Bucket", "S3", u.Host)
	case "gs":
		return mrn.New("Bucket", "GCS", u.Host)
	case "abfss", "abfs", "wasbs", "wasb":
		// abfss://<container>@<account>.dfs.core.windows.net/<path>
		if u.User == nil {
			return ""
		}
		return mrn.New("Container", "AzureBlob", u.User.Username())
	default:
		return ""
	}
}
