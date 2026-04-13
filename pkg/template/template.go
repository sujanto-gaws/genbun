package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sujanto-gaws/genbun/pkg/generator"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TemplateManager handles loading and executing templates
type TemplateManager struct {
	templates    *template.Template
	templatePath string
	funcMap      template.FuncMap
}

// NewTemplateManager creates a new template manager
func NewTemplateManager(templatePath string) (*TemplateManager, error) {
	caser := cases.Title(language.English)

	tm := &TemplateManager{
		templatePath: templatePath,
		funcMap: template.FuncMap{
			"toLower":   toLower,
			"toUpper":   toUpper,
			"title":     caser.String,
			"camelCase": toCamelCase,
			"snakeCase": toSnakeCase,
			"comment":   formatComment,
			"plural":    toPlural,
		},
	}

	// Load templates
	if err := tm.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return tm, nil
}

// loadTemplates loads all template files from the template directory
func (tm *TemplateManager) loadTemplates() error {
	// Check if template path exists
	if _, err := os.Stat(tm.templatePath); os.IsNotExist(err) {
		// Try to use embedded default templates
		return fmt.Errorf("template directory not found: %s", tm.templatePath)
	}

	// Parse all template files
	tmpl, err := template.New("genbun").Funcs(tm.funcMap).ParseGlob(filepath.Join(tm.templatePath, "*.tmpl"))
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	tm.templates = tmpl
	return nil
}

// ExecuteTemplate executes the main model template with the given data
func (tm *TemplateManager) ExecuteTemplate(data generator.ModelTemplate) ([]byte, error) {
	if tm.templates == nil {
		return nil, fmt.Errorf("templates not loaded")
	}

	// Execute the main model template
	var buf bytes.Buffer
	err := tm.templates.ExecuteTemplate(&buf, "model.tmpl", data)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// TemplateExists checks if a specific template file exists
func (tm *TemplateManager) TemplateExists(name string) bool {
	path := filepath.Join(tm.templatePath, name)
	_, err := os.Stat(path)
	return err == nil
}

// GetTemplateFiles returns list of available template files
func (tm *TemplateManager) GetTemplateFiles() []string {
	files, err := filepath.Glob(filepath.Join(tm.templatePath, "*.tmpl"))
	if err != nil {
		return []string{}
	}

	// Convert to just filenames
	result := make([]string, len(files))
	for i, f := range files {
		result[i] = filepath.Base(f)
	}

	return result
}

// Helper functions
func toLower(s string) string {
	return strings.ToLower(s)
}

func toUpper(s string) string {
	return strings.ToUpper(s)
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	var result string
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
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
