package cmd

import (
	"fmt"

	"github.com/altinn/altinn-platform/daisctl/pkg/altinn"
	"github.com/altinn/altinn-platform/daisctl/pkg/kube"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/charmbracelet/bubbles/table"
)

var deployCmd = &cobra.Command{
	Use:   "deployments",
	Short: "Display deployments information",
	RunE: func(cmd *cobra.Command, args []string) error {

		d, err := altinn.GetAllDeployments()
		if err != nil {
			return err
		}

		fmt.Println(setupTable(d.Apps).View())
		return nil
	},
}

func setupTable(deployments map[string]*kube.AppVersions) table.Model {

	columns := []table.Column{
		{Title: "App", Width: 30},
		{Title: "at21", Width: 10},
		{Title: "at22", Width: 10},
		{Title: "at23", Width: 10},
		{Title: "at24", Width: 10},
		{Title: "tt02", Width: 10},
		{Title: "production", Width: 10},
	}
	var rows []table.Row

	for _, d := range deployments {
		rows = append(rows, table.Row{
			d.AppName,
			d.Versions["at21"],
			d.Versions["at22"],
			d.Versions["at23"],
			d.Versions["at24"],
			d.Versions["tt02"],
			d.Versions["production"],
		})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(len(rows)+1),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("241")).
		BorderBottom(true).
		Bold(true)
	t.SetStyles(s)
	return t
}
