# Quick Start: Custom Templates

Get started with custom templates in 5 minutes!

## Step 1: View Default Templates

```bash
# See what templates exist
ls templates/

# View the main template
cat templates/model.tmpl
```

## Step 2: Copy Templates to Your Project

```bash
# Copy to your project directory
cp -r templates /path/to/your/project/my-templates
```

## Step 3: Customize Templates

### Example: Add JSON Tags

Edit `my-templates/struct.tmpl`:

```go
type {{ .ModelName }} struct {
{{- range .Fields }}
	{{ .Name }} {{ .Type }} {{ .Tag }} `json:"{{ .Name }}"`
{{- end }}
}
```

### Example: Add Lifecycle Hooks

Edit `my-templates/methods.tmpl`:

```go
func (m *{{ .ModelName }}) BeforeInsert(ctx context.Context) error {
	// Your custom logic
	return nil
}
```

## Step 4: Generate with Custom Templates

```bash
# Using CLI flag
genbun generate -T ./my-templates

# Or update genbun.yaml
# templates:
#   path: ./my-templates

genbun generate
```

## Step 5: Verify Output

Check your generated models:

```bash
cat internal/model/user.go
```

You should see your customizations!

## Common Customizations

### 1. Add JSON Tags

```go
{{- range .Fields }}
	{{ .Name }} {{ .Type }} {{ .Tag }} `json:"{{ .Name }}"`
{{- end }}
```

### 2. Add Validation

```go
{{- range .Fields }}
{{- if not .IsNullable }}
	{{ .Name }} {{ .Type }} {{ .Tag }} validate:"required"
{{- else }}
	{{ .Name }} {{ .Type }} {{ .Tag }}
{{- end }}
{{- end }}
```

### 3. Add Soft Delete

In `methods.tmpl`:

```go
func (m *{{ .ModelName }}) SoftDelete(db bun.IDB) error {
	_, err := db.NewUpdate().
		Model(m).
		Set("deleted_at = ?", time.Now()).
		WherePK().
		Exec(context.Background())
	return err
}
```

### 4. Add GraphQL Support

```go
type {{ .ModelName }} struct {
{{- range .Fields }}
	{{ .Name }} {{ .Type }} {{ .Tag }} graphql:"{{ .Name }}"
{{- end }}
}
```

## Next Steps

- Read [TEMPLATES.md](TEMPLATES.md) for complete documentation
- Check [example_schema.sql](example_schema.sql) for relationship examples
- Share your templates with the community!
