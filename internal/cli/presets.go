package cli

import (
	"fmt"

	"elk-local/internal/environment"

	"github.com/spf13/cobra"
)

func newPresetsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "presets",
		Short: "List the built-in environment presets",
		Run: func(cmd *cobra.Command, args []string) {
			for _, preset := range environment.ListPresets() {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\tphp=%s\tweb=%s\tdb=%s:%s\t%s\n",
					preset.Name,
					preset.PHPVersion,
					preset.WebServer,
					preset.DatabaseEngine,
					preset.DatabaseVersion,
					preset.Description,
				)
			}
		},
	}
}
