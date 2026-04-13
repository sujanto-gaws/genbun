package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/sujanto-gaws/genbun/pkg/config"
	"github.com/sujanto-gaws/genbun/pkg/database"
	"github.com/sujanto-gaws/genbun/pkg/generator"
)

var (
	outputPath   string
	outputPkg    string
	tables       []string
	excludeTable []string
	schema       string
	dsn          string
	templatePath string
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Bun models from database schema",
	Long: `Connect to a PostgreSQL database and generate Go models with Bun ORM tags.
You can specify tables to include/exclude, output directory, and other options
via command-line flags or configuration file.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			log.Fatalf("Error loading configuration: %v", err)
		}

		// Override config with CLI flags if provided
		if outputPath != "" {
			cfg.Output.Path = outputPath
		}
		if outputPkg != "" {
			cfg.Output.Package = outputPkg
		}
		if len(tables) > 0 {
			cfg.Generate.Tables = tables
		}
		if len(excludeTable) > 0 {
			cfg.Generate.ExcludeTables = excludeTable
		}
		if schema != "" {
			cfg.Generate.Schema = schema
		}
		if templatePath != "" {
			cfg.Templates.Path = templatePath
		}
		if dsn != "" {
			// Parse DSN into database config
			cfg.Database.User = os.Getenv("GENBUN_DB_USER")
			cfg.Database.Password = os.Getenv("GENBUN_DB_PASSWORD")
			cfg.Database.Host = os.Getenv("GENBUN_DB_HOST")
			cfg.Database.Port = 5432
			cfg.Database.DBName = os.Getenv("GENBUN_DB_NAME")
		}

		// Validate required fields
		if cfg.Database.Host == "" || cfg.Database.DBName == "" {
			fmt.Println("Error: database connection settings are required")
			fmt.Println("Use --config flag or set GENBUN_ environment variables")
			os.Exit(1)
		}

		if verbose {
			log.Printf("Connecting to database: %s:%d/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
			log.Printf("Output directory: %s", cfg.Output.Path)
			log.Printf("Output package: %s", cfg.Output.Package)
			log.Printf("Template path: %s", cfg.Templates.Path)
		}

		// Create schema reader
		reader, err := database.NewSchemaReader(cfg.Database.DSN())
		if err != nil {
			log.Fatalf("Error connecting to database: %v", err)
		}
		defer reader.Close()

		// Read tables
		ctx := context.Background()
		tableInfos, err := reader.ReadTables(ctx, cfg)
		if err != nil {
			log.Fatalf("Error reading database schema: %v", err)
		}

		if len(tableInfos) == 0 {
			fmt.Println("No tables found to generate models.")
			return
		}

		if verbose {
			log.Printf("Found %d tables to process", len(tableInfos))
		}

		// Generate models
		gen := generator.NewModelGenerator(cfg, cfg.Templates.Path)
		if err := gen.Generate(tableInfos); err != nil {
			log.Fatalf("Error generating models: %v", err)
		}

		fmt.Printf("\nSuccessfully generated models for %d tables in %s\n", len(tableInfos), cfg.Output.Path)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	// Add flags
	generateCmd.Flags().StringVar(&outputPath, "output", "", "output directory for generated models")
	generateCmd.Flags().StringVar(&outputPkg, "package", "", "package name for generated models")
	generateCmd.Flags().StringSliceVarP(&tables, "tables", "t", nil, "comma-separated list of tables to generate (default: all)")
	generateCmd.Flags().StringSliceVarP(&excludeTable, "exclude", "e", nil, "comma-separated list of tables to exclude")
	generateCmd.Flags().StringVarP(&schema, "schema", "s", "public", "database schema to read")
	generateCmd.Flags().StringVar(&dsn, "dsn", "", "database connection string (DSN)")
	generateCmd.Flags().StringVarP(&templatePath, "templates", "T", "", "template directory path")
}
