package cmd

import (
	"fmt"

	"github.com/altinn/altinn-platform/daisctl/internal/version"
	"github.com/spf13/cobra"
)

var BuildInfo version.VersionInfo

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the build information for daisctl",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Dais version: %s\nCommit: %s\nBuild Date: %s\n", BuildInfo.Version, BuildInfo.Commit, BuildInfo.Date)
		return nil
	},
}
