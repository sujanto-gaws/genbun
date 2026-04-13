package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/sujanto-gaws/genbun/pkg/config"
	"github.com/sujanto-gaws/genbun/pkg/database"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ModelGenerator generates Bun models from database schema
type ModelGenerator struct {
	cfg            *config.Config
	funcMap        template.FuncMap
	templatePath   string
	useExternalTpl bool
}

// ModelField represents a field in the generated model
type ModelField struct {
	Name      string
	Type      string
	Tag       string
	Comment   string
	IsPrimary bool
}

// ModelTemplate represents the data for generating a model file
type ModelTemplate struct {
	PackageName string
	TableName   string
	ModelName   string
	Fields      []ModelField
	Comment     string
	GeneratedAt time.Time
	HasTime     bool
}

// NewModelGenerator creates a new ModelGenerator instance
func NewModelGenerator(cfg *config.Config, templatePath string) *ModelGenerator {
	caser := cases.Title(language.English)

	g := &ModelGenerator{
		cfg:          cfg,
		templatePath: templatePath,
		funcMap: template.FuncMap{
			"toLower":   strings.ToLower,
			"toUpper":   strings.ToUpper,
			"title":     caser.String,
			"camelCase": toCamelCase,
			"snakeCase": toSnakeCase,
			"comment":   formatComment,
			"plural":    toPlural,
		},
	}

	// Check if external templates exist
	if _, err := os.Stat(templatePath); err == nil {
		g.useExternalTpl = true
	}

	return g
}

// Generate generates Bun models for all tables
func (g *ModelGenerator) Generate(tables []database.TableInfo) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(g.cfg.Output.Path, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	var (
		wg    sync.WaitGroup
		errCh = make(chan error, len(tables))
		sem   = make(chan struct{}, 10) // limit concurrency
	)

	for _, table := range tables {
		wg.Add(1)
		go func(t database.TableInfo) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release
			if err := g.generateModel(t); err != nil {
				errCh <- fmt.Errorf("failed to generate model for table %s: %w", t.TableName, err)
			}
		}(table)
	}

	wg.Wait()
	close(errCh)

	// Return first error if any
	for err := range errCh {
		return err
	}

	return nil
}

// generateModel generates a single model file for a table
func (g *ModelGenerator) generateModel(table database.TableInfo) error {
	modelName := g.getModelName(table.TableName)
	templateData := ModelTemplate{
		PackageName: g.cfg.Output.Package,
		TableName:   table.TableName,
		ModelName:   modelName,
		Comment:     table.Comment,
		GeneratedAt: time.Now(),
	}

	// Generate fields
	for _, col := range table.Columns {
		field := g.createField(col, table.PrimaryKey)
		templateData.Fields = append(templateData.Fields, field)

		// Track if we need time import
		if field.Type == "time.Time" || field.Type == "*time.Time" {
			templateData.HasTime = true
		}
	}

	// Generate the file
	var content []byte
	var err error

	if g.useExternalTpl {
		content, err = g.renderExternalTemplate(templateData)
	} else {
		content, err = g.renderTemplate(templateData)
	}

	if err != nil {
		return err
	}

	// Format the Go code
	formatted, err := format.Source(content)
	if err != nil {
		// Log the error but don't fail - write unformatted code
		log.Printf("Warning: could not format generated code: %v", err)
		formatted = content
	}

	// Write the file
	filename := g.getFilename(table.TableName)
	filepath := filepath.Join(g.cfg.Output.Path, filename)

	// Check if file exists and overwrite is disabled
	if !g.cfg.Output.Overwrite {
		if _, err := os.Stat(filepath); err == nil {
			log.Printf("Skipping %s (file already exists, overwrite disabled)", filename)
			return nil
		}
	}

	if err := os.WriteFile(filepath, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("Generated: %s", filepath)
	return nil
}

// createField creates a ModelField from a database column
func (g *ModelGenerator) createField(col database.ColumnInfo, primaryKey []string) ModelField {
	fieldName := toCamelCase(col.Name)
	goType := g.mapPostgresTypeToGo(col)

	// Handle nullable fields
	if col.IsNullable {
		goType = "*" + goType
	}

	// Build Bun tag
	var tagParts []string
	tagParts = append(tagParts, col.Name)

	// Add type tag for custom types
	if col.UDTName != "" && g.isCustomType(col.UDTName) {
		tagParts = append(tagParts, fmt.Sprintf(`bun_type:"%s"`, col.UDTName))
	}

	// Add primary key tag
	isPrimary := false
	for _, pk := range primaryKey {
		if pk == col.Name {
			isPrimary = true
			tagParts = append(tagParts, "pk", "autoincrement")
			break
		}
	}

	tag := strings.Join(tagParts, ",")

	return ModelField{
		Name:      fieldName,
		Type:      goType,
		Tag:       tag,
		Comment:   col.Comment,
		IsPrimary: isPrimary,
	}
}

// mapPostgresTypeToGo maps PostgreSQL types to Go types
func (g *ModelGenerator) mapPostgresTypeToGo(col database.ColumnInfo) string {
	dataType := strings.ToLower(col.DataType)
	udtName := strings.ToLower(col.UDTName)

	// Check UDT name first (more specific)
	switch udtName {
	case "bool", "boolean":
		return "bool"
	case "int2", "smallint":
		return "int16"
	case "int4", "integer", "serial":
		return "int32"
	case "int8", "bigint", "bigserial":
		return "int64"
	case "float4", "real":
		return "float32"
	case "float8", "double precision":
		return "float64"
	case "numeric", "decimal":
		if col.NumericPrecision.Valid && col.NumericPrecision.Int64 <= 18 {
			return "int64"
		}
		return "float64"
	case "date":
		return "time.Time"
	case "timestamp", "timestamp without time zone", "timestamptz", "timestamp with time zone":
		return "time.Time"
	case "time without time zone", "time with time zone":
		return "time.Time"
	case "uuid":
		return "string"
	case "json", "jsonb":
		return "[]byte"
	case "bytea", "blob":
		return "[]byte"
	case "inet", "cidr", "macaddr":
		return "string"
	case "hstore":
		return "[]byte"
	case "interval":
		return "string"
	}

	// Fall back to data type
	switch dataType {
	case "integer", "int", "serial":
		return "int32"
	case "bigint", "bigserial":
		return "int64"
	case "smallint", "smallserial":
		return "int16"
	case "numeric", "decimal":
		return "float64"
	case "real", "double precision":
		return "float64"
	case "boolean":
		return "bool"
	case "character varying", "varchar", "text", "character", "char":
		return "string"
	case "date", "timestamp", "timestamp with time zone", "timestamp without time zone":
		return "time.Time"
	case "time":
		return "time.Time"
	case "uuid":
		return "string"
	case "json", "jsonb":
		return "[]byte"
	case "bytea":
		return "[]byte"
	default:
		// Default to string for unknown types
		return "string"
	}
}

// isCustomType checks if a type is a custom PostgreSQL type
func (g *ModelGenerator) isCustomType(udtName string) bool {
	customTypes := []string{
		"enum", "range", "composite",
	}

	for _, t := range customTypes {
		if strings.Contains(strings.ToLower(udtName), t) {
			return true
		}
	}

	return false
}

// getModelName converts a table name to a Go struct name
func (g *ModelGenerator) getModelName(tableName string) string {
	// Remove prefix if configured
	name := tableName
	if g.cfg.Output.Prefix != "" {
		name = strings.TrimPrefix(name, g.cfg.Output.Prefix)
	}

	// Add suffix if configured
	if g.cfg.Output.Suffix != "" {
		name = name + g.cfg.Output.Suffix
	}

	return toCamelCase(name)
}

// getFilename generates a filename for a table
func (g *ModelGenerator) getFilename(tableName string) string {
	modelName := g.getModelName(tableName)
	return toSnakeCase(modelName) + ".go"
}

// renderTemplate renders the Go struct template
func (g *ModelGenerator) renderTemplate(data ModelTemplate) ([]byte, error) {
	tmpl, err := template.New("model").Funcs(g.funcMap).Parse(modelTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// renderExternalTemplate loads and renders templates from external files
func (g *ModelGenerator) renderExternalTemplate(data ModelTemplate) ([]byte, error) {
	// Parse all template files
	tmpl, err := template.New("genbun").Funcs(g.funcMap).ParseGlob(filepath.Join(g.templatePath, "*.tmpl"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse external templates: %w", err)
	}

	// Execute the main model template
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "model.tmpl", data); err != nil {
		return nil, fmt.Errorf("failed to execute external template: %w", err)
	}

	return buf.Bytes(), nil
}
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	var result string
	for _, part := range parts {
		result += cases.Title(language.English).String(strings.ToLower(part))
	}
	return result
}

func toSnakeCase(s string) string {
	var result string
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result += "_"
		}
		result += strings.ToLower(string(r))
	}
	return result
}

func formatComment(comment string) string {
	if comment == "" {
		return ""
	}
	return "// " + comment
}

func toPlural(s string) string {
	// Simple pluralization - add 's' or 'ies'
	if strings.HasSuffix(s, "y") {
		return s[:len(s)-1] + "ies"
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") || strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	return s + "s"
}

// modelTemplate is the Go template for generating model files
const modelTemplate = `// Code generated by genbun. DO NOT EDIT.
// Generated at {{ .GeneratedAt }}
{{ if .Comment }}
// {{ .Comment }}
{{ end }}

package {{ .PackageName }}
{{ if .HasTime }}

import (
	"time"
)
{{ end }}

// {{ .ModelName }} represents the {{ .TableName }} table.
{{ if .Comment }}// {{ .Comment }}
{{ end }}
type {{ .ModelName }} struct {
{{ range .Fields }}	{{ .Name }} {{ .Type }} {{ .Tag }}
{{ if .Comment }}// {{ .Comment }}
{{ end }}
{{ end }}
{{ range .Relations }}
	// {{ .Comment }}
	{{ .Name }} {{ .Type }} {{ .Tag }}
{{ end }}
}

// TableName overrides the default table name.
func (m *{{ .ModelName }}) TableName() string {
	return "{{ .TableName }}"
}
`
