package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "1.0.0"
	cfgFile   string
	verbose   bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "genbun",
	Short: "Generate Bun ORM models from PostgreSQL database schema",
	Long: `genbun is a CLI tool that generates Go models for the Bun ORM
from existing PostgreSQL database schemas. It automatically reads table
structures, columns, primary keys, and relationships to generate
idiomatic Go code.`,
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./genbun.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}
