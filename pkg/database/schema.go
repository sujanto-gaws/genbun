package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sujanto-gaws/genbun/pkg/config"
)

// ColumnInfo represents metadata about a database column
type ColumnInfo struct {
	Name             string
	DataType         string
	IsNullable       bool
	ColumnDefault    sql.NullString
	CharacterLength  sql.NullInt64
	NumericPrecision sql.NullInt64
	UDTName          string
	IsIdentity       bool
	Comment          string
}

// ForeignKeyInfo represents a foreign key relationship
type ForeignKeyInfo struct {
	Name             string
	ColumnName       string
	ReferencedTable  string
	ReferencedColumn string
	ReferencedSchema string
}

// RelationInfo represents complete relationship metadata
type RelationInfo struct {
	Type             string // "belongs_to", "has_many", "many_to_many"
	Name             string // Go field name
	TargetModel      string // Target Go type name
	TargetTable      string // Target table name
	ForeignKey       string // FK column name (for belongs_to)
	JoinTable        string // Join table (for many_to_many)
	JoinForeignKey   string // Join table FK to this model
	TargetForeignKey string // Join table FK to target model
	Through          string // Through model (for many_to_many)
	BelongsTo        bool
	HasMany          bool
	ManyToMany       bool
}

// TableInfo represents complete metadata about a database table
type TableInfo struct {
	SchemaName  string
	TableName   string
	Columns     []ColumnInfo
	PrimaryKey  []string
	Comment     string
	ForeignKeys []ForeignKeyInfo
	Relations   []RelationInfo
}

// SchemaReader reads database schema information from PostgreSQL
type SchemaReader struct {
	db *sql.DB
}

// NewSchemaReader creates a new SchemaReader instance
func NewSchemaReader(dsn string) (*SchemaReader, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool for concurrent queries
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &SchemaReader{db: db}, nil
}

// Close closes the database connection
func (r *SchemaReader) Close() error {
	return r.db.Close()
}

// ReadTables reads schema information for specified tables
func (r *SchemaReader) ReadTables(ctx context.Context, cfg *config.Config) ([]TableInfo, error) {
	tables, err := r.getTableList(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get table list: %w", err)
	}

	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		tableInfos = make([]TableInfo, len(tables))
		idx        = 0
		sem        = make(chan struct{}, 5) // limit concurrent DB queries
	)

	for _, table := range tables {
		wg.Add(1)
		go func(tableName string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release
			info, err := r.readTableInfo(ctx, cfg.Generate.Schema, tableName)
			if err != nil {
				log.Printf("Warning: failed to read table info for %s: %v", tableName, err)
				return
			}
			mu.Lock()
			tableInfos[idx] = info
			idx++
			mu.Unlock()
		}(table)
	}

	wg.Wait()
	tableInfos = tableInfos[:idx]

	return tableInfos, nil
}

// getTableList returns the list of tables to process
func (r *SchemaReader) getTableList(ctx context.Context, cfg *config.Config) ([]string, error) {
	if len(cfg.Generate.Tables) > 0 {
		return cfg.Generate.Tables, nil
	}

	// Get all tables from the schema
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = $1 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	rows, err := r.db.QueryContext(ctx, query, cfg.Generate.Schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}

		// Check if table should be excluded
		shouldExclude := false
		for _, excluded := range cfg.Generate.ExcludeTables {
			if excluded == tableName {
				shouldExclude = true
				break
			}
		}

		if !shouldExclude {
			tables = append(tables, tableName)
		}
	}

	return tables, nil
}

// readTableInfo reads complete metadata for a specific table
func (r *SchemaReader) readTableInfo(ctx context.Context, schema, tableName string) (TableInfo, error) {
	info := TableInfo{
		SchemaName: schema,
		TableName:  tableName,
	}

	// Read columns
	columns, err := r.readColumns(ctx, schema, tableName)
	if err != nil {
		return info, err
	}
	info.Columns = columns

	// Read primary key
	pk, err := r.readPrimaryKey(ctx, schema, tableName)
	if err != nil {
		return info, err
	}
	info.PrimaryKey = pk

	// Read table comment
	comment, err := r.readTableComment(ctx, schema, tableName)
	if err != nil {
		log.Printf("Warning: failed to read table comment for %s: %v", tableName, err)
	}
	info.Comment = comment

	// Read foreign keys
	foreignKeys, err := r.readForeignKeys(ctx, schema, tableName)
	if err != nil {
		log.Printf("Warning: failed to read foreign keys for %s: %v", tableName, err)
	}
	info.ForeignKeys = foreignKeys

	return info, nil
}

// readColumns reads column metadata for a table
func (r *SchemaReader) readColumns(ctx context.Context, schema, tableName string) ([]ColumnInfo, error) {
	// Build the qualified table name in Go to avoid PostgreSQL parameter type issues
	qualifiedTable := fmt.Sprintf("%s.%s", schema, tableName)
	query := `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			c.character_maximum_length,
			c.numeric_precision,
			c.udt_name,
			CASE WHEN c.is_identity = 'YES' THEN true ELSE false END as is_identity,
			COALESCE(pg_catalog.col_description(
				$1::regclass::oid,
				c.ordinal_position
			), '') as comment
		FROM information_schema.columns c
		WHERE c.table_schema = $2
		AND c.table_name = $3
		ORDER BY c.ordinal_position`

	rows, err := r.db.QueryContext(ctx, query, qualifiedTable, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var isNullable, isIdentity string

		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&isNullable,
			&col.ColumnDefault,
			&col.CharacterLength,
			&col.NumericPrecision,
			&col.UDTName,
			&isIdentity,
			&col.Comment,
		)
		if err != nil {
			return nil, err
		}

		col.IsNullable = isNullable == "YES"
		col.IsIdentity = isIdentity == "YES"

		columns = append(columns, col)
	}

	return columns, nil
}

// readPrimaryKey reads the primary key columns for a table
func (r *SchemaReader) readPrimaryKey(ctx context.Context, schema, tableName string) ([]string, error) {
	query := `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		AND tc.table_schema = $1
		AND tc.table_name = $2
		ORDER BY kcu.ordinal_position`

	rows, err := r.db.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pk []string
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return nil, err
		}
		pk = append(pk, colName)
	}

	return pk, nil
}

// readTableComment reads the comment/description for a table
func (r *SchemaReader) readTableComment(ctx context.Context, schema, tableName string) (string, error) {
	// Build the qualified table name in Go to avoid PostgreSQL parameter type issues
	qualifiedTable := fmt.Sprintf("%s.%s", schema, tableName)
	query := `
		SELECT COALESCE(
			pg_catalog.obj_description(
				$1::regclass::oid,
				'pg_class'
			),
			''
		)`

	var comment string
	err := r.db.QueryRowContext(ctx, query, qualifiedTable).Scan(&comment)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(comment), nil
}

// readForeignKeys reads foreign key constraints for a table
func (r *SchemaReader) readForeignKeys(ctx context.Context, schema, tableName string) ([]ForeignKeyInfo, error) {
	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_schema,
			ccu.table_name,
			ccu.column_name
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON tc.constraint_name = ccu.constraint_name
			AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		AND tc.table_schema = $1
		AND tc.table_name = $2`

	rows, err := r.db.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []ForeignKeyInfo
	for rows.Next() {
		var fk ForeignKeyInfo
		if err := rows.Scan(&fk.Name, &fk.ColumnName, &fk.ReferencedSchema, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}

	return fks, nil
}

// detectRelationships analyzes tables and creates relationship metadata
func (r *SchemaReader) detectRelationships(tables []TableInfo, cfg *config.Config) []TableInfo {
	// Create a map for quick table lookup
	tableMap := make(map[string]*TableInfo)
	for i := range tables {
		tableMap[tables[i].TableName] = &tables[i]
	}

	// Detect relationships for each table
	for i := range tables {
		r.detectChanges(&tables[i], tableMap)
	}

	return tables
}

// detectBelongsTo detects belongs-to relationships from foreign keys
func (r *SchemaReader) detectBelongsTo(table *TableInfo, tableMap map[string]*TableInfo) {
	for _, fk := range table.ForeignKeys {
		targetTable, exists := tableMap[fk.ReferencedTable]
		if !exists {
			continue
		}

		// Check if this relationship already exists
		alreadyExists := false
		for _, rel := range table.Relations {
			if rel.ForeignKey == fk.ColumnName {
				alreadyExists = true
				break
			}
		}
		if alreadyExists {
			continue
		}

		relation := RelationInfo{
			Type:        "belongs_to",
			BelongsTo:   true,
			Name:        stripIDSuffix(toCamelCase(fk.ColumnName)), // FK column name with _id removed
			ForeignKey:  fk.ColumnName,
			TargetTable: fk.ReferencedTable,
			TargetModel: toCamelCase(fk.ReferencedTable),
		}

		table.Relations = append(table.Relations, relation)

		// Add has-many relationship to the target table
		hasManyRelation := RelationInfo{
			Type:        "has_many",
			HasMany:     true,
			Name:        toPlural(toCamelCase(table.TableName)), // Use source table name (plural)
			ForeignKey:  fk.ColumnName,
			TargetTable: table.TableName,
			TargetModel: toCamelCase(table.TableName),
		}

		// Check if this relationship already exists
		exists_in_target := false
		for _, rel := range targetTable.Relations {
			if rel.Type == "has_many" && rel.TargetTable == table.TableName {
				exists_in_target = true
				break
			}
		}

		if !exists_in_target {
			targetTable.Relations = append(targetTable.Relations, hasManyRelation)
		}
	}
}

// detectManyToMany detects many-to-many relationships through join tables
func (r *SchemaReader) detectManyToMany(table *TableInfo, tableMap map[string]*TableInfo) {
	// A join table typically has exactly 2 foreign keys and a composite primary key
	if len(table.ForeignKeys) < 2 {
		return
	}

	// Check if this table could be a join table
	// (has 2 FKs and PK is composite of those FK columns)
	if len(table.PrimaryKey) >= 2 {
		// Track already-added relations by a composite key: target + joinFK + targetFK
		added := make(map[string]bool)
		for _, fk := range table.ForeignKeys {
			if targetTable, exists := tableMap[fk.ReferencedTable]; exists {
				// Add many-to-many relationship
				for _, otherFK := range table.ForeignKeys {
					if otherFK.ColumnName == fk.ColumnName {
						continue
					}

					if otherTarget, exists := tableMap[otherFK.ReferencedTable]; exists {
						// Unique key for dedup: join table + target + both FK columns
						dupKey := table.TableName + "|" + otherTarget.TableName + "|" + fk.ColumnName + "|" + otherFK.ColumnName
						if added[dupKey] {
							continue
						}
						added[dupKey] = true

						relation := RelationInfo{
							Type:             "many_to_many",
							ManyToMany:       true,
							Name:             toPlural(toCamelCase(otherTarget.TableName)), // Plural for many-to-many
							TargetTable:      otherTarget.TableName,
							TargetModel:      toCamelCase(otherTarget.TableName),
							JoinTable:        table.TableName,
							JoinForeignKey:   fk.ColumnName,
							TargetForeignKey: otherFK.ColumnName,
							Through:          toCamelCase(table.TableName),
						}
						targetTable.Relations = append(targetTable.Relations, relation)
					}
				}
			}
		}
	}
}

// detectChanges runs all relationship detection
func (r *SchemaReader) detectChanges(table *TableInfo, tableMap map[string]*TableInfo) {
	r.detectBelongsTo(table, tableMap)
}

// Helper functions
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

// stripIDSuffix removes common ID suffixes from a field name.
func stripIDSuffix(s string) string {
	suffixes := []string{"ID", "Id"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			s = s[:len(s)-len(suffix)]
			break
		}
	}
	return s
}
