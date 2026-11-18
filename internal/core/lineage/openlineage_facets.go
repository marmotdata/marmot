package lineage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
)

// This file turns OpenLineage facets into what Marmot stores about an asset. A facet missing from one event never clears what an earlier event said, because events describe a run piecemeal; mergeContribution keeps the earlier values.

// datasetFields is what one event says about a dataset.
func datasetFields(ds *Dataset, id datasetIdentity) asset.ContributionFields {
	name := id.Name
	fields := asset.ContributionFields{
		Name:      &name,
		Type:      id.Type,
		Providers: []string{id.Provider},
		Metadata:  map[string]interface{}{"namespace": ds.Namespace, "openlineage_name": ds.Name},
		Tags:      []string{"openlineage", strings.ToLower(id.Type), strings.ToLower(id.Provider)},
	}
	f := ds.Facets
	if doc := facetMap(f, "documentation"); doc != nil {
		if d, _ := doc["description"].(string); d != "" {
			fields.Description = &d
		}
	}
	if s := facetMap(f, "schema"); s != nil {
		fields.Schema = schemaFromFacet(s)
	}
	addOwners(fields.Metadata, facetMap(f, "ownership"))
	fields.Tags = append(fields.Tags, tagsFromFacet(facetMap(f, "tags"))...)
	if dt := facetMap(f, "datasetType"); dt != nil {
		setString(fields.Metadata, "dataset_type", dt["datasetType"])
		setString(fields.Metadata, "dataset_subtype", dt["subType"])
	}
	if st := facetMap(f, "storage"); st != nil {
		setString(fields.Metadata, "storage_layer", st["storageLayer"])
		setString(fields.Metadata, "file_format", st["fileFormat"])
	}
	if v := facetMap(f, "version"); v != nil {
		setString(fields.Metadata, "version", v["datasetVersion"])
	}
	if src := facetMap(f, "dataSource"); src != nil {
		if uri, _ := src["uri"].(string); cleanURI(uri) != "" {
			fields.Metadata["data_source"] = cleanURI(uri)
		}
		if n, _ := src["name"].(string); n != "" && !strings.Contains(n, "://") {
			fields.Metadata["data_source_name"] = n
		}
	}
	if c := facetMap(f, "catalog"); c != nil {
		setString(fields.Metadata, "catalog_framework", c["framework"])
		setString(fields.Metadata, "catalog_type", c["type"])
		setString(fields.Metadata, "catalog_name", c["name"])
	}
	if lc := facetMap(f, "lifecycleStateChange"); lc != nil {
		setString(fields.Metadata, "lifecycle_state", lc["lifecycleStateChange"])
	}
	if dq := facetMap(f, "dataQualityMetrics"); dq != nil {
		setNumber(fields.Metadata, "row_count", dq["rowCount"])
		setNumber(fields.Metadata, "size_bytes", dq["bytes"])
		setNumber(fields.Metadata, "file_count", dq["fileCount"])
	}
	if q := facetMap(f, "sql"); q != nil {
		if query, _ := q["query"].(string); query != "" {
			lang := "sql"
			fields.Query, fields.QueryLanguage = &query, &lang
		}
	}
	if in := facetMap(ds.InputFacets, "inputStatistics"); in != nil {
		setNumber(fields.Metadata, "input_row_count", in["rowCount"])
		setNumber(fields.Metadata, "input_size_bytes", in["size"])
		setNumber(fields.Metadata, "input_file_count", in["fileCount"])
	}
	if dq := facetMap(ds.InputFacets, "dataQualityMetrics"); dq != nil {
		setNumber(fields.Metadata, "row_count", dq["rowCount"])
		setNumber(fields.Metadata, "size_bytes", dq["bytes"])
		setNumber(fields.Metadata, "file_count", dq["fileCount"])
	}
	if out := facetMap(ds.OutputFacets, "outputStatistics"); out != nil {
		setNumber(fields.Metadata, "output_row_count", out["rowCount"])
		setNumber(fields.Metadata, "output_size_bytes", out["size"])
		// duck_lineage sends the byte count as sizeInBytes rather than the spec's size.
		setNumber(fields.Metadata, "output_size_bytes", out["sizeInBytes"])
		setNumber(fields.Metadata, "output_file_count", out["fileCount"])
	}
	fields.Tags = dedupe(fields.Tags)
	return fields
}

// jobFields is what one event says about the job that ran.
func jobFields(event *RunEvent, id jobIdentity) asset.ContributionFields {
	name := id.Name
	desc := fmt.Sprintf("%s from %s namespace", id.Type, event.Job.Namespace)
	fields := asset.ContributionFields{
		Name:      &name,
		Type:      id.Type,
		Providers: []string{id.Provider},
		Metadata: map[string]interface{}{
			"openlineage_producer": event.Producer,
			"job_type":             id.Type,
			"namespace":            event.Job.Namespace,
			"job_name":             event.Job.Name,
		},
		Tags: []string{"openlineage", strings.ToLower(id.Type), strings.ToLower(id.Provider)},
	}
	f := event.Job.Facets
	if doc := facetMap(f, "documentation"); doc != nil {
		if d, _ := doc["description"].(string); d != "" {
			desc = d
		}
	}
	fields.Description = &desc
	if q := facetMap(f, "sql"); q != nil {
		if query, _ := q["query"].(string); query != "" {
			lang := "sql"
			fields.Query, fields.QueryLanguage = &query, &lang
		}
		setString(fields.Metadata, "sql_dialect", q["dialect"])
	}
	if sc := facetMap(f, "sourceCode"); sc != nil {
		if code, _ := sc["sourceCode"].(string); code != "" {
			lang, _ := sc["language"].(string)
			lang = strings.ToLower(lang)
			if lang == "" {
				lang = "text"
			}
			fields.Query, fields.QueryLanguage = &code, &lang
		}
	}
	if loc := facetMap(f, "sourceCodeLocation"); loc != nil {
		setString(fields.Metadata, "source_code_url", loc["url"])
		setString(fields.Metadata, "source_code_repo_url", loc["repoUrl"])
		setString(fields.Metadata, "source_code_path", loc["path"])
		setString(fields.Metadata, "source_code_version", loc["version"])
		setString(fields.Metadata, "source_code_branch", loc["branch"])
		setString(fields.Metadata, "source_code_tag", loc["tag"])
	}
	if jt := facetMap(f, "jobType"); jt != nil {
		setString(fields.Metadata, "processing_type", jt["processingType"])
		setString(fields.Metadata, "integration", jt["integration"])
	}
	addOwners(fields.Metadata, facetMap(f, "ownership"))
	fields.Tags = append(fields.Tags, tagsFromFacet(facetMap(f, "tags"))...)
	if airflow := facetMap(f, "airflow"); airflow != nil {
		if tasks, ok := airflow["tasks"].(map[string]interface{}); ok && len(tasks) > 0 {
			names := make([]string, 0, len(tasks))
			for taskName := range tasks {
				names = append(names, taskName)
			}
			sort.Strings(names)
			fields.Metadata["tasks"] = names
		}
	}
	fields.Tags = dedupe(fields.Tags)
	return fields
}

// addOwners records every owner the ownership facet names, and the first as owner, which earlier releases wrote.
func addOwners(metadata map[string]interface{}, ownership map[string]interface{}) {
	if ownership == nil {
		return
	}
	list, _ := ownership["owners"].([]interface{})
	var names []string
	for _, o := range list {
		if m, ok := o.(map[string]interface{}); ok {
			if n, _ := m["name"].(string); n != "" {
				names = append(names, n)
			}
		}
	}
	if len(names) > 0 {
		metadata["owner"] = names[0]
		metadata["owners"] = names
	}
}

// tagsFromFacet turns a tags facet into Marmot tags: key:value, or the key alone when the value is true. A tag scoped to one field is about a column, not the asset, so it is left out.
func tagsFromFacet(tags map[string]interface{}) []string {
	if tags == nil {
		return nil
	}
	list, _ := tags["tags"].([]interface{})
	var out []string
	for _, t := range list {
		m, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		if field, _ := m["field"].(string); field != "" {
			continue
		}
		key, _ := m["key"].(string)
		value, _ := m["value"].(string)
		if key == "" {
			continue
		}
		if value == "" || strings.EqualFold(value, "true") {
			out = append(out, key)
		} else {
			out = append(out, key+":"+value)
		}
	}
	return out
}

func setString(m map[string]interface{}, key string, v interface{}) {
	if s, ok := v.(string); ok && s != "" {
		m[key] = s
	}
}

func setNumber(m map[string]interface{}, key string, v interface{}) {
	switch n := v.(type) {
	case float64:
		m[key] = int64(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			m[key] = i
		}
	case int, int64:
		m[key] = n
	}
}

func dedupe(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// mergeContribution lays what an event says over what OpenLineage said about the asset before, so a facet one event leaves out keeps its earlier value. Metadata is merged key by key and tags accumulate.
func mergeContribution(prev *asset.ContributionFields, next asset.ContributionFields) asset.ContributionFields {
	if prev == nil {
		return next
	}
	out := next
	if out.Description == nil {
		out.Description = prev.Description
	}
	if len(out.Schema) == 0 {
		out.Schema = prev.Schema
	}
	if out.Query == nil {
		out.Query, out.QueryLanguage = prev.Query, prev.QueryLanguage
	}
	if len(out.ExternalLinks) == 0 {
		out.ExternalLinks = prev.ExternalLinks
	}
	metadata := make(map[string]interface{}, len(prev.Metadata)+len(next.Metadata))
	for k, v := range prev.Metadata {
		// Earlier releases recorded which side of the last event the dataset was on, which flipped from event to event.
		if k == "openlineage_role" {
			continue
		}
		metadata[k] = v
	}
	for k, v := range next.Metadata {
		metadata[k] = v
	}
	out.Metadata = metadata
	out.Tags = dedupe(append(slices.DeleteFunc(slices.Clone(prev.Tags), func(t string) bool { return t == "input" || t == "output" }), next.Tags...))
	return out
}

// schemaColumn is one column in the list table plugins store under the columns schema, which is the form the catalog reads everywhere.
type schemaColumn struct {
	Name        string `json:"column_name"`
	Type        string `json:"data_type"`
	Description string `json:"description,omitempty"`
}

// schemaFromFacet turns an OpenLineage schema facet into the columns schema table plugins write, keeping the producer's type names. Columns follow ordinal_position when every field has one and the producer's order otherwise. Nested fields stay out of the list, since the parent's type already describes them. It returns nil when the facet names no fields, so an event without them leaves the schema alone.
func schemaFromFacet(schemaData map[string]interface{}) map[string]string {
	fields, ok := schemaData["fields"].([]interface{})
	if !ok {
		return nil
	}
	type positioned struct {
		col schemaColumn
		pos float64
	}
	columns := make([]positioned, 0, len(fields))
	allPositioned := true
	for _, field := range fields {
		fieldMap, ok := field.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := fieldMap["name"].(string)
		if name == "" {
			continue
		}
		column := schemaColumn{Name: name, Type: "unknown"}
		if fieldType, _ := fieldMap["type"].(string); fieldType != "" {
			column.Type = fieldType
		}
		column.Description, _ = fieldMap["description"].(string)
		pos, ok := fieldMap["ordinal_position"].(float64)
		if !ok {
			allPositioned = false
		}
		columns = append(columns, positioned{col: column, pos: pos})
	}
	if len(columns) == 0 {
		return nil
	}
	if allPositioned {
		sort.SliceStable(columns, func(i, j int) bool { return columns[i].pos < columns[j].pos })
	}
	out := make([]schemaColumn, len(columns))
	for i, c := range columns {
		out[i] = c.col
	}
	// Types such as STRUCT<city STRING> stay readable in the raw schema rather than escaped for HTML.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return nil
	}
	return map[string]string{"columns": strings.TrimSpace(buf.String())}
}
