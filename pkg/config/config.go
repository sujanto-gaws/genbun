package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the complete configuration for genbun
type Config struct {
	Database  DatabaseConfig `mapstructure:"database"`
	Output    OutputConfig   `mapstructure:"output"`
	Generate  GenerateConfig `mapstructure:"generate"`
	Templates TemplateConfig `mapstructure:"templates"`
}

// DatabaseConfig holds PostgreSQL database connection settings
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// OutputConfig controls where and how generated files are written
type OutputConfig struct {
	Path      string `mapstructure:"path"`
	Package   string `mapstructure:"package"`
	Overwrite bool   `mapstructure:"overwrite"`
	Prefix    string `mapstructure:"prefix"`
	Suffix    string `mapstructure:"suffix"`
}

// GenerateConfig controls generation behavior
type GenerateConfig struct {
	Tables        []string `mapstructure:"tables"`
	ExcludeTables []string `mapstructure:"exclude_tables"`
	Schema        string   `mapstructure:"schema"`
	WithHooks     bool     `mapstructure:"with_hooks"`
	WithTimes     bool     `mapstructure:"with_times"`
}

// TemplateConfig controls template settings
type TemplateConfig struct {
	Path   string `mapstructure:"path"`
	Custom string `mapstructure:"custom"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Database: DatabaseConfig{
			Host:    "localhost",
			Port:    5432,
			SSLMode: "disable",
		},
		Output: OutputConfig{
			Path:      "internal/model",
			Package:   "model",
			Overwrite: true,
		},
		Generate: GenerateConfig{
			Schema:    "public",
			WithHooks: true,
			WithTimes: true,
		},
		Templates: TemplateConfig{
			Path: "templates",
		},
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("genbun")
	v.SetConfigType("yaml")

	// Set defaults
	def := DefaultConfig()
	v.SetDefault("database.host", def.Database.Host)
	v.SetDefault("database.port", def.Database.Port)
	v.SetDefault("database.sslmode", def.Database.SSLMode)
	v.SetDefault("output.path", def.Output.Path)
	v.SetDefault("output.package", def.Output.Package)
	v.SetDefault("output.overwrite", def.Output.Overwrite)
	v.SetDefault("generate.schema", def.Generate.Schema)
	v.SetDefault("generate.with_hooks", def.Generate.WithHooks)
	v.SetDefault("generate.with_times", def.Generate.WithTimes)
	v.SetDefault("templates.path", def.Templates.Path)

	// Determine config file path
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Search in current directory and home directory
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(home)
		}
		v.AddConfigPath(".")
	}

	// Read config file if it exists
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Environment variable support
	v.SetEnvPrefix("GENBUN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Ensure output path is absolute
	if !filepath.IsAbs(cfg.Output.Path) {
		cfg.Output.Path = filepath.Join(".", cfg.Output.Path)
	}

	return &cfg, nil
}

// DSN returns the PostgreSQL connection string
func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User,
		d.Password,
		d.Host,
		d.Port,
		d.DBName,
		d.SSLMode,
	)
}

// WriteConfig creates a default config file
func WriteConfig(path string) error {
	cfg := DefaultConfig()

	v := viper.New()
	v.Set("database", cfg.Database)
	v.Set("output", cfg.Output)
	v.Set("generate", cfg.Generate)
	v.Set("templates", cfg.Templates)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory: %w", err)
		}
	}

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}
