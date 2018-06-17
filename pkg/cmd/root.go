package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ghg",
	Short: "A better GitHub issue search",
	Long:  `Search through GitHub issues and comments offline`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return searchCmd.RunE(cmd, args)
	},
}

// Execute is the entry point into the `ghg` cli.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
