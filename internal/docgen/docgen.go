package docgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"
)

const markerPrefix = "+marmot:"

const docTemplate = `---
title: {{ .Name }}
description: {{ .Description }}
status: {{ .Status }}
---

# {{ .Name }}

<div class="flex flex-col gap-3 mb-6 pb-6 border-b border-gray-200">
<div class="flex items-center gap-3">
{{ if eq .Status "stable" }}<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-green-300 text-earthy-green-900">Stable</span>{{ else if eq .Status "beta" }}<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-blue-300 text-earthy-blue-900">Beta</span>{{ else }}<span class="inline-flex items-center rounded-full px-4 py-2 text-sm font-medium bg-earthy-yellow-300 text-earthy-yellow-900">Experimental</span>{{ end }}
</div>
{{if .Features}}<div class="flex items-center gap-2">
<span class="text-sm text-gray-500">Creates:</span>
<div class="flex flex-wrap gap-2">{{range .Features}}<span class="inline-flex items-center rounded-lg px-4 py-2 text-sm font-medium bg-earthy-green-100 text-earthy-green-800 border border-earthy-green-300">{{ . }}</span>{{end}}</div>
</div>{{end}}
</div>

import { CalloutCard } from '@site/src/components/DocCard';

<CalloutCard
  title="Configure in the UI"
  description="This plugin can be configured directly in the Marmot UI with a step-by-step wizard."
  href="/docs/Populating/UI"
  buttonText="View Guide"
  variant="secondary"
  icon="mdi:cursor-default-click"
/>
{{if .AdditionalContent}}

{{ .AdditionalContent }}{{end}}
{{if .SupportedServices}}
## Supported Services{{range .SupportedServices}}
- {{ . }}{{end}}{{end}}{{if .ExampleConfig}}

## Example Configuration

` + "```yaml" + `
{{ .ExampleConfig }}
` + "```" + `{{end}}{{if .ConfigProperties}}

## Configuration
{{ if .ConfigDescription }}
{{ .ConfigDescription }}

{{end}}The following configuration options are available:

| Property | Type | Required | Description |
|----------|------|----------|-------------|{{range .ConfigProperties}}
| {{ .Name }} | {{ .Type }} | {{ .Required }} | {{ .Description }} |{{end}}{{end}}{{if .MetadataFields}}

## Available Metadata

The following metadata fields are available:

| Field | Type | Description |
|-------|------|-------------|{{range .MetadataFields}}
| {{ .Name }} | {{ .Type }} | {{ .Description }} |{{end}}{{end}}{{range .AdditionalSections}}

## {{ .Title }}

{{ .Content }}{{end}}`

type PluginDoc struct {
	Name               string
	Description        string
	ConfigDescription  string
	ConfigProperties   []PropertyDoc
	MetadataFields     []PropertyDoc
	SupportedServices  []string
	ExampleConfig      string
	Status             string
	Features           []string
	AdditionalContent  string
	AdditionalSections []AdditionalSection
	configSource       string // relative path to source plugin for whitelabel plugins
}

type PropertyDoc struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

type AdditionalSection struct {
	Title   string
	Content string
}

// TypeRegistry holds all discovered types for resolution
type TypeRegistry struct {
	types map[string]*ast.TypeSpec
	files map[string]*ast.File
}

func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types: make(map[string]*ast.TypeSpec),
		files: make(map[string]*ast.File),
	}
}

func (tr *TypeRegistry) addFile(file *ast.File) {
	tr.files[file.Name.Name] = file

	// Extract all type declarations
	ast.Inspect(file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					// Store both local name and package-qualified name
					tr.types[typeSpec.Name.Name] = typeSpec
					if file.Name != nil {
						tr.types[file.Name.Name+"."+typeSpec.Name.Name] = typeSpec
					}
				}
			}
		}
		return true
	})
}

func (tr *TypeRegistry) resolveType(typeName string) *ast.TypeSpec {
	// Try direct lookup first
	if typeSpec, ok := tr.types[typeName]; ok {
		return typeSpec
	}

	// Try without package prefix if it exists
	parts := strings.Split(typeName, ".")
	if len(parts) > 1 {
		simpleName := parts[len(parts)-1]
		if typeSpec, ok := tr.types[simpleName]; ok {
			return typeSpec
		}
		// Also try with the package prefix
		packageName := parts[0]
		if typeSpec, ok := tr.types[packageName+"."+simpleName]; ok {
			return typeSpec
		}
	}

	return nil
}

func GeneratePluginDocs(pluginPath string, outputDir string) error {
	fset := token.NewFileSet()
	pluginDoc := &PluginDoc{}
	registry := NewTypeRegistry()

	// First pass: collect all type definitions from plugin directory
	err := filepath.Walk(pluginPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return fmt.Errorf("parsing file %s: %w", path, err)
			}
			registry.addFile(file)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("walking plugin directory for types: %w", err)
	}

	// Also scan for common plugin types in parent directories
	pluginParent := filepath.Dir(pluginPath)
	for i := 0; i < 3; i++ { // Look up to 3 levels up
		pluginDir := filepath.Join(pluginParent, "plugin")
		if _, err := os.Stat(pluginDir); err == nil {
			err := filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.HasSuffix(path, ".go") {
					file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
					if err != nil {
						return nil // Skip files that don't parse
					}
					registry.addFile(file)
				}
				return nil
			})
			if err == nil {
				break // Found and processed plugin directory
			}
		}
		pluginParent = filepath.Dir(pluginParent)
	}

	// Second pass: process files for documentation
	err = filepath.Walk(pluginPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return fmt.Errorf("parsing file %s: %w", path, err)
			}

			processFile(pluginDoc, file, registry)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking plugin directory: %w", err)
	}

	if pluginDoc.Name == "" {
		return fmt.Errorf("no plugin documentation found")
	}

	// If no config was found locally, check for config-source marker pointing to another plugin
	if len(pluginDoc.ConfigProperties) == 0 && pluginDoc.configSource != "" {
		sourcePath := filepath.Join(pluginPath, pluginDoc.configSource)
		sourcePath, _ = filepath.Abs(sourcePath)
		fmt.Printf("Loading config from source plugin: %s\n", sourcePath)

		sourceDoc := &PluginDoc{}
		sourceRegistry := NewTypeRegistry()

		// Scan the source plugin directory for types
		_ = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return nil
			}
			sourceRegistry.addFile(file)
			return nil
		})

		// Also scan the plugin package for embedded types (e.g. BaseConfig)
		pluginParentSearch := filepath.Dir(sourcePath)
		for i := 0; i < 3; i++ {
			pluginPkgDir := filepath.Join(pluginParentSearch, "plugin")
			if _, statErr := os.Stat(pluginPkgDir); statErr == nil {
				_ = filepath.Walk(pluginPkgDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
						return err
					}
					file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
					if err != nil {
						return nil
					}
					sourceRegistry.addFile(file)
					return nil
				})
				break
			}
			pluginParentSearch = filepath.Dir(pluginParentSearch)
		}

		// Process source files for config and metadata
		_ = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return nil
			}
			processFile(sourceDoc, file, sourceRegistry)
			return nil
		})

		pluginDoc.ConfigProperties = sourceDoc.ConfigProperties
		pluginDoc.MetadataFields = sourceDoc.MetadataFields
		if pluginDoc.ExampleConfig == "" {
			pluginDoc.ExampleConfig = sourceDoc.ExampleConfig
		}
	}

	pluginDoc.AdditionalContent = loadAdditionalMarkdown(pluginDoc.Name, outputDir)

	// Remove duplicates and sort
	pluginDoc.ConfigProperties = removeDuplicateProperties(pluginDoc.ConfigProperties)
	sort.Slice(pluginDoc.ConfigProperties, func(i, j int) bool {
		return pluginDoc.ConfigProperties[i].Name < pluginDoc.ConfigProperties[j].Name
	})

	docsDir := filepath.Join(outputDir, "Plugins")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("creating docs directory: %w", err)
	}

	fileName := filepath.Join(docsDir, pluginDoc.Name+".md")
	fmt.Printf("Writing documentation to: %s\n", fileName)

	return writeDoc(pluginDoc, fileName)
}

func loadAdditionalMarkdown(pluginName string, docsOutputDir string) string {
	// Go up one level from docsOutputDir and then into plugin-docs
	baseDir := filepath.Dir(docsOutputDir)
	pluginDocsDir := filepath.Join(baseDir, "plugin-docs")

	fmt.Printf("docsOutputDir: %s\n", docsOutputDir)
	fmt.Printf("baseDir: %s\n", baseDir)
	fmt.Printf("pluginDocsDir: %s\n", pluginDocsDir)

	entries, err := os.ReadDir(pluginDocsDir)
	if err != nil {
		fmt.Printf("Error reading directory %s: %v\n", pluginDocsDir, err)
		return ""
	}

	fmt.Printf("Files found in %s:\n", pluginDocsDir)
	for _, entry := range entries {
		fmt.Printf("  - %s\n", entry.Name())
	}

	targetFileName := strings.ToLower(pluginName + ".md")
	fmt.Printf("Looking for file: %s\n", targetFileName)

	for _, entry := range entries {
		if !entry.IsDir() && strings.ToLower(entry.Name()) == targetFileName {
			markdownFile := filepath.Join(pluginDocsDir, entry.Name())
			fmt.Printf("Found match: %s\n", markdownFile)
			content, err := os.ReadFile(markdownFile)
			if err != nil {
				fmt.Printf("Error reading file: %v\n", err)
				return ""
			}
			fmt.Printf("Successfully loaded %d bytes\n", len(content))
			return string(content)
		}
	}

	fmt.Printf("No matching file found\n")
	return ""
}

func removeDuplicateProperties(properties []PropertyDoc) []PropertyDoc {
	seen := make(map[string]bool)
	var result []PropertyDoc

	for _, prop := range properties {
		if !seen[prop.Name] {
			seen[prop.Name] = true
			result = append(result, prop)
		}
	}

	return result
}

func parseMarkers(text string) map[string]string {
	markers := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, markerPrefix) {
			marker := strings.TrimPrefix(line, markerPrefix)
			parts := strings.SplitN(marker, "=", 2)
			key := strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				markers[key] = strings.TrimSpace(parts[1])
			} else {
				markers[key] = ""
			}
		}
	}
	return markers
}

func processFile(pluginDoc *PluginDoc, file *ast.File, registry *TypeRegistry) {
	// Package docs
	if file.Doc != nil {
		markers := parseMarkers(file.Doc.Text())
		if name, ok := markers["name"]; ok {
			pluginDoc.Name = name
		}
		if desc, ok := markers["description"]; ok {
			pluginDoc.Description = desc
		}
		if status, ok := markers["status"]; ok {
			pluginDoc.Status = status
		}
		if features, ok := markers["features"]; ok {
			for _, f := range strings.Split(features, ",") {
				f = strings.TrimSpace(f)
				if f != "" {
					pluginDoc.Features = append(pluginDoc.Features, f)
				}
			}
		}
		if source, ok := markers["config-source"]; ok {
			pluginDoc.configSource = source
		}
	}

	// Process all declarations
	ast.Inspect(file, func(n ast.Node) bool {
		d, ok := n.(*ast.GenDecl)
		if ok {
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						// Process config type
						hasConfigMarker := false
						if d.Doc != nil {
							hasConfigMarker = strings.Contains(d.Doc.Text(), "+marmot:config")
						}
						if ts.Doc != nil {
							hasConfigMarker = hasConfigMarker || strings.Contains(ts.Doc.Text(), "+marmot:config")
						}

						if hasConfigMarker || ts.Name.Name == "Config" {
							if st, ok := ts.Type.(*ast.StructType); ok {
								pluginDoc.ConfigProperties = processStructFieldsWithRegistry(st, registry, make(map[string]bool))
							}
						}

						// Process metadata
						hasMetadataMarker := false
						if d.Doc != nil {
							hasMetadataMarker = strings.Contains(d.Doc.Text(), "+marmot:metadata")
						}
						if ts.Doc != nil {
							hasMetadataMarker = hasMetadataMarker || strings.Contains(ts.Doc.Text(), "+marmot:metadata")
						}

						if hasMetadataMarker {
							if st, ok := ts.Type.(*ast.StructType); ok {
								fields := processStructFieldsWithRegistry(st, registry, make(map[string]bool))
								pluginDoc.MetadataFields = append(pluginDoc.MetadataFields, fields...)
							}
						}
					}
				}
			} else if d.Tok == token.VAR || d.Tok == token.CONST {
				for _, spec := range d.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok || len(valueSpec.Values) == 0 {
						continue
					}

					// Look for example config marker at both levels
					hasExampleMarker := false
					if d.Doc != nil {
						hasExampleMarker = strings.Contains(d.Doc.Text(), "+marmot:example-config")
					}
					if valueSpec.Doc != nil {
						hasExampleMarker = hasExampleMarker || strings.Contains(valueSpec.Doc.Text(), "+marmot:example-config")
					}

					if hasExampleMarker {
						if lit, ok := valueSpec.Values[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							pluginDoc.ExampleConfig = strings.Trim(lit.Value, "`")
						}
					}
				}
			}
		}
		return true
	})

	// Sort metadata fields
	sort.Slice(pluginDoc.MetadataFields, func(i, j int) bool {
		return pluginDoc.MetadataFields[i].Name < pluginDoc.MetadataFields[j].Name
	})
}

func processStructFieldsWithRegistry(st *ast.StructType, registry *TypeRegistry, visited map[string]bool) []PropertyDoc {
	var fields []PropertyDoc

	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			typeName := parseFieldTypeForLookup(field.Type)
			fmt.Printf("Processing embedded field: %s\n", typeName)
			if typeName != "" && !visited[typeName] {
				visited[typeName] = true
				if resolvedType := registry.resolveType(typeName); resolvedType != nil {
					fmt.Printf("  Resolved %s successfully\n", typeName)
					if embeddedStruct, ok := resolvedType.Type.(*ast.StructType); ok {
						embeddedFields := processStructFieldsWithRegistry(embeddedStruct, registry, visited)
						fields = append(fields, embeddedFields...)
					}
				} else {
					fmt.Printf("  Failed to resolve %s\n", typeName)
				}
			}
			continue
		}

		var fieldType string
		var jsonName string
		var description string
		var required bool
		var name string

		fieldType = parseFieldType(field.Type)

		if field.Doc != nil {
			desc := field.Doc.Text()
			desc = strings.TrimPrefix(desc, "//")
			desc = strings.TrimSpace(desc)
			description = desc
		}

		if field.Tag != nil {
			tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
			if jsonTag := tag.Get("json"); jsonTag != "" && jsonTag != "-" {
				parts := strings.Split(jsonTag, ",")
				jsonName = parts[0]

				for _, part := range parts {
					if part == "inline" {
						typeName := parseFieldTypeForLookup(field.Type)
						fmt.Printf("Processing inline field: %s\n", typeName)
						if typeName != "" && !visited[typeName] {
							visited[typeName] = true
							if resolvedType := registry.resolveType(typeName); resolvedType != nil {
								fmt.Printf("  Resolved %s successfully\n", typeName)
								if inlineStruct, ok := resolvedType.Type.(*ast.StructType); ok {
									inlineFields := processStructFieldsWithRegistry(inlineStruct, registry, visited)
									fields = append(fields, inlineFields...)
								}
							} else {
								fmt.Printf("  Failed to resolve %s\n", typeName)
							}
						}
						goto nextField
					}
				}
			}
			if tagDesc := tag.Get("description"); tagDesc != "" {
				description = tagDesc
			}
			required = tag.Get("required") == "true"
		}

		name = jsonName
		if name == "" {
			name = field.Names[0].Name
		}

		fields = append(fields, PropertyDoc{
			Name:        name,
			Type:        fieldType,
			Description: description,
			Required:    required,
		})

	nextField:
	}

	return fields
}

// parseFieldTypeForLookup extracts the type name for registry lookup
func parseFieldTypeForLookup(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return parseFieldTypeForLookup(t.X)
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)
		}
	}
	return ""
}

func parseFieldType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.ArrayType:
		return "[]" + parseFieldType(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", parseFieldType(t.Key), parseFieldType(t.Value))
	case *ast.StarExpr:
		return parseFieldType(t.X)
	case *ast.StructType:
		return "struct"
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name)
		}
		return "interface{}"
	default:
		return "interface{}"
	}
}

func writeDoc(doc *PluginDoc, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	tmpl, err := template.New("plugin-doc").Parse(docTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	if err := tmpl.Execute(f, doc); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	return nil
}
