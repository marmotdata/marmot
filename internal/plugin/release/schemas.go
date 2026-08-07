package release

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// AssetSchema describes one metadata struct a plugin attaches to
// discovered assets. It matches the JSON shape the registry consumes.
type AssetSchema struct {
	StructName  string       `json:"struct_name"`
	DisplayName string       `json:"display_name"`
	Description string       `json:"description,omitempty"`
	Fields      []AssetField `json:"fields"`
}

// AssetField is one field in an AssetSchema.
type AssetField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// ScanAssetSchemas walks root and returns an AssetSchema for every
// top-level struct whose name ends in "Fields". Field names come from
// `metadata:"..."` tags and descriptions from `description:"..."` tags.
// The scan reads source only — plugins are not compiled — so it is
// safe to run against arbitrary plugin trees at release time.
func ScanAssetSchemas(root string) ([]AssetSchema, error) {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	out := []AssetSchema{}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				name := ts.Name.Name
				if !strings.HasSuffix(name, "Fields") || seen[name] {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := collectAssetFields(st)
				if len(fields) == 0 {
					continue
				}
				seen[name] = true
				out = append(out, AssetSchema{
					StructName:  name,
					DisplayName: assetDisplayName(name),
					Description: docText(pickDoc(gd, ts)),
					Fields:      fields,
				})
			}
		}
		return nil
	})
	return out, err
}

func collectAssetFields(st *ast.StructType) []AssetField {
	var out []AssetField
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		raw, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		tag := reflect.StructTag(raw)
		name := tag.Get("metadata")
		if name == "" {
			continue
		}
		out = append(out, AssetField{
			Name:        name,
			Type:        astTypeString(f.Type),
			Description: tag.Get("description"),
		})
	}
	return out
}

// astTypeString collapses a field type to "string", "int", "float",
// "bool", or the source name for named types, with "[]" per slice level.
// Qualified names (pkg.T) surface as "T"; the package prefix is dropped
// because consumers rarely care and it keeps the schema shape stable
// across renamed imports.
func astTypeString(expr ast.Expr) string {
	var suffix string
	for {
		arr, ok := expr.(*ast.ArrayType)
		if !ok {
			break
		}
		suffix += "[]"
		expr = arr.Elt
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return primitiveOrName(t.Name) + suffix
	case *ast.SelectorExpr:
		return t.Sel.Name + suffix
	default:
		return "object" + suffix
	}
}

func primitiveOrName(name string) string {
	switch name {
	case "string", "bool":
		return name
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune":
		return "int"
	case "float32", "float64":
		return "float"
	default:
		return name
	}
}

func pickDoc(gd *ast.GenDecl, ts *ast.TypeSpec) *ast.CommentGroup {
	if ts.Doc != nil {
		return ts.Doc
	}
	return gd.Doc
}

// docText joins a doc comment's prose lines, skipping `+marmot:...`
// directive lines.
func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	var lines []string
	for _, c := range cg.List {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)
		if text == "" || strings.HasPrefix(text, "+") {
			continue
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, " ")
}

// assetDisplayName turns "PostgresColumnFields" into "Postgres Column":
// trailing "Fields" is dropped and CamelCase split on word boundaries,
// keeping acronym runs glued ("AWSGlue" → "AWS Glue").
func assetDisplayName(structName string) string {
	n := strings.TrimSuffix(structName, "Fields")
	var b strings.Builder
	runes := []rune(n)
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			boundary := (unicode.IsLower(prev) && unicode.IsUpper(r)) ||
				(unicode.IsUpper(prev) && unicode.IsUpper(r) && i+1 < len(runes) && unicode.IsLower(runes[i+1]))
			if boundary {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
