package release

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanAssetSchemas(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "types.go"), `
package fixture

// PostgresFields holds PostgreSQL asset metadata.
// +marmot:metadata
type PostgresFields struct {
	Host string ` + "`json:\"host\" metadata:\"host\" description:\"Hostname\"`" + `
	Port int    ` + "`json:\"port\" metadata:\"port\"`" + `

	// unexportedTagged is picked up too: the AST scanner doesn't
	// filter by export status, matching how MapToMetadata behaves at
	// runtime.
	unexportedTagged string ` + "`metadata:\"internal_only\"`" + `
}

// AWSGlueTableFields checks acronym runs stay together.
type AWSGlueTableFields struct {
	Name string ` + "`metadata:\"name\"`" + `
}

// NoTags is skipped because it has no fields carrying a metadata tag.
type NoTagsFields struct {
	Whatever string ` + "`json:\"whatever\"`" + `
}

// Config is not a *Fields struct, so it's skipped even though it has
// metadata tags.
type Config struct {
	Host string ` + "`metadata:\"host\"`" + `
}
`)

	// A vendor dir must be skipped even if it contains matching structs.
	writeFile(t, filepath.Join(dir, "vendor", "ignored.go"), `
package ignored
type VendorFields struct {
	X string `+"`metadata:\"x\"`"+`
}
`)

	// _test.go files must be skipped.
	writeFile(t, filepath.Join(dir, "types_test.go"), `
package fixture
type TestOnlyFields struct {
	X string `+"`metadata:\"x\"`"+`
}
`)

	schemas, err := ScanAssetSchemas(dir)
	require.NoError(t, err)

	names := make(map[string]AssetSchema, len(schemas))
	for _, s := range schemas {
		names[s.StructName] = s
	}
	assert.Contains(t, names, "PostgresFields")
	assert.Contains(t, names, "AWSGlueTableFields")
	assert.NotContains(t, names, "NoTagsFields")
	assert.NotContains(t, names, "Config", "non-Fields struct must be skipped")
	assert.NotContains(t, names, "VendorFields", "vendor dir must be skipped")
	assert.NotContains(t, names, "TestOnlyFields", "_test.go files must be skipped")

	pg := names["PostgresFields"]
	assert.Equal(t, "Postgres", pg.DisplayName)
	assert.Equal(t, "PostgresFields holds PostgreSQL asset metadata.", pg.Description)
	assert.Equal(t, []AssetField{
		{Name: "host", Type: "string", Description: "Hostname"},
		{Name: "port", Type: "int"},
		{Name: "internal_only", Type: "string"},
	}, pg.Fields)

	glue := names["AWSGlueTableFields"]
	assert.Equal(t, "AWS Glue Table", glue.DisplayName)
}

func TestAstTypeString(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "types.go"), `
package x
type MixedFields struct {
	Str      string    `+"`metadata:\"str\"`"+`
	Num      int64     `+"`metadata:\"num\"`"+`
	Ratio    float64   `+"`metadata:\"ratio\"`"+`
	Ok       bool      `+"`metadata:\"ok\"`"+`
	Tags     []string  `+"`metadata:\"tags\"`"+`
	Nested2D [][]int   `+"`metadata:\"nested_2d\"`"+`
	Ptr      *string   `+"`metadata:\"ptr\"`"+`
	Opaque   any       `+"`metadata:\"opaque\"`"+`
	Custom   FooBar    `+"`metadata:\"custom\"`"+`
}
`)
	schemas, err := ScanAssetSchemas(dir)
	require.NoError(t, err)
	require.Len(t, schemas, 1)
	byName := map[string]AssetField{}
	for _, f := range schemas[0].Fields {
		byName[f.Name] = f
	}
	assert.Equal(t, "string", byName["str"].Type)
	assert.Equal(t, "int", byName["num"].Type)
	assert.Equal(t, "float", byName["ratio"].Type)
	assert.Equal(t, "bool", byName["ok"].Type)
	assert.Equal(t, "string[]", byName["tags"].Type)
	assert.Equal(t, "int[][]", byName["nested_2d"].Type)
	assert.Equal(t, "string", byName["ptr"].Type)
	assert.Equal(t, "any", byName["opaque"].Type)
	assert.Equal(t, "FooBar", byName["custom"].Type)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
