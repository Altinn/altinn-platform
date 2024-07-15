package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// TODO: use real versions
const appVersion = "0.0.1"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of my-cli",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("dais version %s\n", appVersion)
		return nil
	},
}
