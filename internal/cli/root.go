package cli

import "github.com/spf13/cobra"

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:           "elk-local",
		Short:         "Manage local PHP development environments",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	command.AddCommand(newCreateCommand())
	command.AddCommand(newSwitchCommand())
	command.AddCommand(newBackupCommand())
	command.AddCommand(newExportCommand())
	command.AddCommand(newImportCommand())
	command.AddCommand(newRestoreCommand())
	command.AddCommand(newStartCommand())
	command.AddCommand(newStopCommand())
	command.AddCommand(newStatusCommand())
	command.AddCommand(newDestroyCommand())
	command.AddCommand(newPresetsCommand())
	command.AddCommand(newVersionCommand())
	command.AddCommand(newDaemonCommand())
	command.AddCommand(newServeCommand())
	command.AddCommand(newProxyCommand())

	return command
}
