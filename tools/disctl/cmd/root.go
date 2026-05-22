package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dis",
	Short: "Disctl is a CLI tool for managing and interacting with the Dis platform",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Completion not needed at the moment
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(releasesCmd)
	rootCmd.AddCommand(versionCmd)
}
