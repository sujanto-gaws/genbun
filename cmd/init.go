package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/sujanto-gaws/genbun/pkg/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new genbun configuration file",
	Long: `Create a default genbun.yaml configuration file in the current directory.
You can then edit this file to configure your database connection and generation options.`,
	Run: func(cmd *cobra.Command, args []string) {
		outputPath := "genbun.yaml"

		// Check if file already exists
		if err := config.WriteConfig(outputPath); err != nil {
			log.Fatalf("Error creating config file: %v", err)
		}

		fmt.Printf("Created configuration file: %s\n", outputPath)
		fmt.Println("Please edit this file to configure your database connection and generation options.")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
