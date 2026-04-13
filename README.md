# genbun - PostgreSQL to Bun ORM Model Generator

A powerful CLI tool that automatically generates Go models with Bun ORM tags from your PostgreSQL database schema, similar to EF Core tools.

## Features

- **Automatic Schema Detection**: Reads tables, columns, primary keys, and constraints from PostgreSQL
- **Relationship Detection**: Automatically detects and generates BelongsTo, HasMany, and ManyToMany relationships
- **External Template System**: Customize generated models using external template files
- **Bun ORM Tags**: Generates proper `bun` struct tags for seamless ORM mapping including relationships
- **Type Mapping**: Intelligent PostgreSQL to Go type conversion
- **Nullable Support**: Properly handles nullable columns with pointer types
- **Customizable**: Flexible configuration via YAML file, environment variables, or CLI flags
- **Clean Output**: Generates properly formatted, idiomatic Go code
- **Table Filtering**: Include or exclude specific tables
- **Multi-Schema Support**: Works with any PostgreSQL schema

## Installation

### From Source

```bash
go install github.com/sujanto-gaws/genbun@latest
```

### Build from Repository

```bash
git clone <repository-url>
cd genbun
go build -o genbun.exe .
```

## Quick Start

### 1. Initialize Configuration

```bash
genbun init
```

This creates a `genbun.yaml` configuration file with default settings.

### 2. Configure Database Connection

Edit `genbun.yaml`:

```yaml
database:
  host: localhost
  port: 5432
  user: your_username
  password: your_password
  dbname: your_database
  sslmode: disable

output:
  path: internal/model
  package: model
  overwrite: true

generate:
  schema: public
  tables: []  # Leave empty to generate all tables
  exclude_tables: []  # Tables to exclude
```

### 3. Generate Models

```bash
genbun generate
```

## Usage

### CLI Commands

#### Initialize Configuration

```bash
genbun init
```

Creates a default configuration file.

#### Generate Models

```bash
genbun generate [flags]
```

### Command-Line Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | | Config file path | `./genbun.yaml` |
| `--output` | | Output directory | (from config) |
| `--package` | | Package name | (from config) |
| `--tables` | `-t` | Tables to generate (comma-separated) | All tables |
| `--exclude` | `-e` | Tables to exclude (comma-separated) | None |
| `--schema` | `-s` | Database schema | `public` |
| `--verbose` | `-v` | Enable verbose output | `false` |

### Examples

**Generate all tables:**
```bash
genbun generate
```

**Generate specific tables:**
```bash
genbun generate -t users,posts,comments
```

**Exclude certain tables:**
```bash
genbun generate -e migrations,schema_versions
```

**Custom output:**
```bash
genbun generate --output pkg/models --package models
```

**Verbose mode:**
```bash
genbun generate -v
```

**Use custom config file:**
```bash
genbun generate --config configs/genbun-prod.yaml
```

## Configuration

### Environment Variables

genbun supports environment variables with the `GENBUN_` prefix:

```bash
export GENBUN_DATABASE_HOST=localhost
export GENBUN_DATABASE_USER=myuser
export GENBUN_DATABASE_PASSWORD=mypass
export GENBUN_DATABASE_DBNAME=mydb
export GENBUN_OUTPUT_PATH=internal/model
```

### Configuration Options

#### Database Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `database.host` | PostgreSQL host | `localhost` |
| `database.port` | PostgreSQL port | `5432` |
| `database.user` | Database user | `` |
| `database.password` | Database password | `` |
| `database.dbname` | Database name | `` |
| `database.sslmode` | SSL mode | `disable` |

#### Output Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `output.path` | Output directory | `internal/model` |
| `output.package` | Package name | `model` |
| `output.overwrite` | Overwrite existing files | `true` |
| `output.prefix` | Prefix to remove from table names | `` |
| `output.suffix` | Suffix to add to model names | `` |

#### Generate Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `generate.tables` | Tables to include | All tables |
| `generate.exclude_tables` | Tables to exclude | `[]` |
| `generate.schema` | Database schema | `public` |

## Type Mapping

genbun automatically maps PostgreSQL types to Go types:

| PostgreSQL Type | Go Type |
|----------------|---------|
| `boolean` | `bool` |
| `smallint` / `int2` | `int16` |
| `integer` / `int4` / `serial` | `int32` |
| `bigint` / `int8` / `bigserial` | `int64` |
| `real` / `float4` | `float32` |
| `double precision` / `float8` | `float64` |
| `numeric` / `decimal` | `int64` or `float64` |
| `varchar` / `text` / `char` | `string` |
| `uuid` | `string` |
| `date` / `timestamp` / `timestamptz` | `time.Time` |
| `time` | `time.Time` |
| `json` / `jsonb` | `[]byte` |
| `bytea` | `[]byte` |
| `inet` / `cidr` | `string` |

Nullable columns are generated as pointer types (e.g., `*string`, `*int64`).

## Relationship Detection

genbun automatically detects relationships from foreign key constraints:

### Relationship Types

| Type | Description | Bun Tag |
|------|-------------|---------|
| **BelongsTo** | Foreign key in this table referencing another table | `bun:"rel:belongs-to,join:fk_column=target.id"` |
| **HasMany** | Other table has foreign key referencing this table | `bun:"rel:has-many,join:id=source.fk_column"` |
| **ManyToMany** | Two tables connected through a join table | `bun:"rel:many-to-many,join:..."` |

### How It Works

1. **BelongsTo**: When a table has a foreign key, genbun generates a field referencing the target model
2. **HasMany**: Automatically added to the referenced table (reverse of BelongsTo)
3. **ManyToMany**: Detected when a table has exactly 2 foreign keys with a composite primary key

### Using Relationships

```go
// Query with relations
user := &model.User{ID: 1}
err := db.NewSelect().Model(user).Relation("Posts").Scan(ctx)

// Access related data
fmt.Printf("User: %s\n", user.Username)
fmt.Printf("Posts: %d\n", len(user.Posts))

// Nested relations
err := db.NewSelect().Model(user).
    Relation("Posts").
    Relation("Posts.Tags").
    Relation("Posts.User").
    Scan(ctx)
```

## Generated Code Example

### Database Schema with Relationships

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    title VARCHAR(200) NOT NULL,
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE tags (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

CREATE TABLE post_tags (
    post_id INTEGER REFERENCES posts(id),
    tag_id INTEGER REFERENCES tags(id),
    PRIMARY KEY (post_id, tag_id)
);
```

### Generated Go Models with Relationships

#### User Model (Has Many Posts)

```go
// Code generated by genbun. DO NOT EDIT.
// Generated at 2026-04-13 10:30:00

package model

import (
	"time"
)

// User represents the users table.
type User struct {
	ID        int32     `bun:"id,pk,autoincrement"`
	Username  string    `bun:"username"`
	Email     string    `bun:"email"`
	CreatedAt time.Time `bun:"created_at"`

	// has_many relationship to posts
	Posts []Post `bun:"rel:has-many,join:id=posts.user_id"`
}

// TableName overrides the default table name.
func (m *User) TableName() string {
	return "users"
}
```

#### Post Model (Belongs To User, ManyToMany Tags)

```go
// Code generated by genbun. DO NOT EDIT.
// Generated at 2026-04-13 10:30:00

package model

import (
	"time"
)

// Post represents the posts table.
type Post struct {
	ID        int32     `bun:"id,pk,autoincrement"`
	UserID    int32     `bun:"user_id"`
	Title     string    `bun:"title"`
	Content   *string   `bun:"content"`
	CreatedAt time.Time `bun:"created_at"`

	// belongs_to relationship to users
	User User `bun:"rel:belongs-to,join:user_id=users.id"`

	// many_to_many relationship to tags
	Tags []Tag `bun:"rel:many-to-many,join:post_id=post_tags.tag_id,join:post_tags.post_id=tags"`
}

// TableName overrides the default table name.
func (m *Post) TableName() string {
	return "posts"
}
```

#### Tag Model (ManyToMany Posts)

```go
// Code generated by genbun. DO NOT EDIT.
// Generated at 2026-04-13 10:30:00

package model

// Tag represents the tags table.
type Tag struct {
	ID   int32  `bun:"id,pk,autoincrement"`
	Name string `bun:"name"`

	// many_to_many relationship to posts
	Posts []Post `bun:"rel:many-to-many,join:tag_id=post_tags.post_id,join:post_tags.tag_id=posts"`
}

// TableName overrides the default table name.
func (m *Tag) TableName() string {
	return "tags"
}
```

## Project Structure

```
genbun/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command
│   ├── init.go            # Init command
│   └── generate.go        # Generate command
├── pkg/
│   ├── config/            # Configuration management
│   │   └── config.go
│   ├── database/          # Database schema reader
│   │   └── schema.go
│   └── generator/         # Model generator
│       └── generator.go
├── main.go                # Entry point
├── go.mod
└── genbun.yaml            # Configuration file
```

## Advanced Usage

### Working with Large Databases

For databases with many tables, use the `--tables` flag to generate specific tables:

```bash
genbun generate -t users,user_roles,roles
```

### Multiple Schemas

If you work with multiple schemas, specify the schema:

```bash
genbun generate --schema analytics
```

### Model Naming

Control model naming with prefix/suffix options in config:

```yaml
output:
  prefix: "tbl_"  # Remove tbl_ prefix from table names
  suffix: "Model" # Add Model suffix to struct names
```

## Troubleshooting

### Connection Issues

- Verify database credentials in `genbun.yaml`
- Check that the database is accessible
- Ensure the user has sufficient permissions to read schema information

### No Tables Generated

- Verify the schema name is correct
- Check that tables exist in the specified schema
- Review `exclude_tables` configuration

### Type Mapping Issues

- Unknown types default to `string`
- Custom types may need manual adjustment
- Check the verbose output for warnings

## Comparison with EF Tools

genbun provides similar functionality to Entity Framework Core tools:

| EF Core Tool | genbun Equivalent |
|--------------|-------------------|
| `dotnet ef dbcontext scaffold` | `genbun generate` |
| `--output-dir` | `--output` |
| `--schema` | `--schema` |
| `--table` | `--tables` |
| `appsettings.json` | `genbun.yaml` |

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is open source and available under the MIT License.

## Support

For issues, questions, or suggestions, please open an issue in the repository.
