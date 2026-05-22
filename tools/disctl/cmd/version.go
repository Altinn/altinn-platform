package cmd

import (
	"fmt"

	"github.com/altinn/altinn-platform/disctl/internal/version"
	"github.com/spf13/cobra"
)

var BuildInfo version.VersionInfo

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the build information for disctl",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Dis version: %s\nCommit: %s\nBuild Date: %s\n", BuildInfo.Version, BuildInfo.Commit, BuildInfo.Date)
		return nil
	},
}
