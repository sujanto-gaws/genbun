# Relationship Detection Guide

This guide explains how genbun detects and generates relationships from your PostgreSQL database schema.

## Overview

genbun automatically analyzes foreign key constraints in your PostgreSQL database and generates appropriate Bun ORM relationship fields in your models.

## Relationship Types

### 1. BelongsTo

**When it's created:** When a table has a foreign key referencing another table.

**Example Schema:**
```sql
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id)
);
```

**Generated Field in Post model:**
```go
// BelongsTo relationship - single model reference
User User `bun:"rel:belongs-to,join:user_id=users.id"`
```

**Usage:**
```go
post := &model.Post{ID: 1}
err := db.NewSelect().Model(post).Relation("User").Scan(ctx)
fmt.Printf("Post by: %s\n", post.User.Username)
```

### 2. HasMany

**When it's created:** Automatically on the referenced table (reverse of BelongsTo).

**Generated Field in User model:**
```go
// HasMany relationship - slice of models
Posts []Post `bun:"rel:has-many,join:id=posts.user_id"`
```

**Usage:**
```go
user := &model.User{ID: 1}
err := db.NewSelect().Model(user).Relation("Posts").Scan(ctx)
fmt.Printf("User has %d posts\n", len(user.Posts))
```

### 3. ManyToMany

**When it's created:** When a join table has exactly 2 foreign keys with a composite primary key.

**Example Schema:**
```sql
CREATE TABLE post_tags (
    post_id INTEGER NOT NULL REFERENCES posts(id),
    tag_id INTEGER NOT NULL REFERENCES tags(id),
    PRIMARY KEY (post_id, tag_id)
);
```

**Generated Field in Post model:**
```go
// ManyToMany relationship through join table
Tags []Tag `bun:"rel:many-to-many,join:post_id=post_tags.tag_id,join:post_tags.post_id=tags"`
```

**Usage:**
```go
post := &model.Post{ID: 1}
err := db.NewSelect().Model(post).Relation("Tags").Scan(ctx)
fmt.Printf("Post has %d tags\n", len(post.Tags))
```

## Detection Rules

### Foreign Key Detection
genbun queries PostgreSQL's `information_schema` to find all foreign key constraints:
- Reads constraint name
- Identifies the foreign key column
- Determines the referenced table and column

### BelongsTo Detection
For each foreign key in a table:
- Creates a field with the target model type
- Names the field after the FK column (camelCase)
- Adds appropriate Bun relation tag

### HasMany Detection
For each BelongsTo relationship found:
- Adds reverse relationship to the target table
- Names the field as plural of the source table
- Creates slice type (`[]Model`)

### ManyToMany Detection
A table is considered a join table if:
- Has exactly 2 or more foreign keys
- Has a composite primary key (2+ columns)
- PK columns match the foreign key columns

## Naming Conventions

### Field Naming
- **BelongsTo**: FK column name in camelCase
  - `user_id` → `UserId` field
  - `category_id` → `CategoryId` field

- **HasMany**: Pluralized target table name in camelCase
  - `posts` → `Posts` field
  - `categories` → `Categories` field

- **ManyToMany**: Pluralized target table name
  - `tags` → `Tags` field
  - `categories` → `Categories` field

### Pluralization Rules
- Words ending in `y` → `ies` (e.g., `Category` → `Categories`)
- Words ending in `s`, `x`, `ch`, `sh` → `es`
- All other words → add `s`

## Complex Examples

### Self-Referencing Relationship

**Schema:**
```sql
CREATE TABLE comments (
    id SERIAL PRIMARY KEY,
    post_id INTEGER REFERENCES posts(id),
    parent_id INTEGER REFERENCES comments(id)
);
```

**Generated:**
```go
type Comment struct {
    ID       int32     `bun:"id,pk,autoincrement"`
    PostID   int32     `bun:"post_id"`
    ParentID *int32    `bun:"parent_id"`
    
    // Self-referencing belongs-to
    Parent Comment `bun:"rel:belongs-to,join:parent_id=comments.id"`
    
    // Self-referencing has-many
    Replies []Comment `bun:"rel:has-many,join:id=comments.parent_id"`
}
```

### One-to-One Relationship

**Schema:**
```sql
CREATE TABLE profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER UNIQUE REFERENCES users(id)
);
```

**Generated:**
```go
// In Profile model
User User `bun:"rel:belongs-to,join:user_id=users.id"`

// In User model (detected as has-many, but effectively one-to-one due to UNIQUE constraint)
Profiles []Profile `bun:"rel:has-many,join:id=profiles.user_id"`
```

## Querying with Relationships

### Basic Relation Query
```go
// Load post with user
post := &model.Post{}
err := db.NewSelect().
    Model(post).
    Relation("User").
    Where("post.id = ?", 1).
    Scan(ctx)
```

### Multiple Relations
```go
// Load post with user and tags
post := &model.Post{}
err := db.NewSelect().
    Model(post).
    Relation("User").
    Relation("Tags").
    Where("post.id = ?", 1).
    Scan(ctx)
```

### Nested Relations
```go
// Load user with posts and their tags
user := &model.User{}
err := db.NewSelect().
    Model(user).
    Relation("Posts").
    Relation("Posts.Tags").
    Where("user.id = ?", 1).
    Scan(ctx)
```

### Conditional Relations
```go
// Load user with only published posts
user := &model.User{}
err := db.NewSelect().
    Model(user).
    Relation("Posts", func(q *bun.SelectQuery) *bun.SelectQuery {
        return q.Where("published_at IS NOT NULL")
    }).
    Where("user.id = ?", 1).
    Scan(ctx)
```

## Troubleshooting

### Relationship Not Detected

**Check:**
1. Foreign key constraint exists in database
2. Referenced table is included in generation
3. Schema name is correct
4. Table is not in `exclude_tables` list

**Solution:**
```bash
# Use verbose mode to see what's being processed
genbun generate -v
```

### Wrong Relationship Type

**Common causes:**
- Missing primary key on join table
- Incorrect foreign key constraints
- Multiple schemas with same table names

**Solution:**
- Ensure join tables have composite primary keys
- Use `--schema` flag to specify correct schema
- Verify foreign keys with:
```sql
SELECT * FROM information_schema.referential_constraints
WHERE table_name = 'your_table';
```

### Naming Issues

**Problem:** Field name doesn't match expected pattern

**Solution:**
- Check column naming convention (snake_case recommended)
- Review prefix/suffix configuration
- Manually edit generated file if needed

## Advanced Configuration

### Exclude Specific Tables
```yaml
generate:
  exclude_tables:
    - migrations
    - schema_versions
    - audit_logs
```

### Include Specific Tables Only
```yaml
generate:
  tables:
    - users
    - posts
    - comments
```

### Custom Schema
```yaml
generate:
  schema: app_data
```

## Best Practices

1. **Always use foreign keys** - genbun can only detect relationships that exist in the database
2. **Use composite primary keys** on join tables for proper ManyToMany detection
3. **Name FK columns consistently** - use `table_name_id` pattern (e.g., `user_id`, `post_id`)
4. **Review generated code** - while automated, you may want to customize relationship names
5. **Use verbose mode** during generation to see detection details

## Manual Customization

After generation, you can customize relationships:

```go
type Post struct {
    ID     int32  `bun:"id,pk"`
    UserID int32  `bun:"user_id"`
    
    // Custom relationship name
    Author User `bun:"rel:belongs-to,join:user_id=users.id"`
    
    // Custom filter on relationship
    PublishedComments []Comment `bun:"rel:has-many,join:id=comments.post_id"`
}
```

Note: Customizations will be overwritten on regeneration unless `output.overwrite: false` is set.
