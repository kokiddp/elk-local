package cli

import (
	"fmt"

	"elk-local/internal/environment"

	"github.com/spf13/cobra"
)

func newBackupCommand() *cobra.Command {
	var projectRoot string
	var includeProjectFiles bool
	var force bool

	command := &cobra.Command{
		Use:   "backup NAME",
		Short: "Create a managed backup archive for an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := loadEnvironmentManifest(projectRoot, args[0])
			if err != nil {
				return err
			}

			artifact, err := environment.NewBackupService(nil).Backup(manifest, environment.BackupCreateOptions{
				IncludeProjectFiles: includeProjectFiles,
				Force:               force,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created backup %s\n", artifact.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "Database: %s\n", artifact.Metadata.DatabaseName)
			fmt.Fprintf(cmd.OutOrStdout(), "Includes project files: %t\n", artifact.Metadata.IncludesProjectFiles)
			return nil
		},
	}

	command.Flags().StringVar(&projectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	command.Flags().BoolVar(&includeProjectFiles, "include-project-files", false, "Include a project snapshot in the backup archive")
	command.Flags().BoolVar(&force, "force", false, "Overwrite an existing archive if the generated path already exists")

	return command
}

func newExportCommand() *cobra.Command {
	var projectRoot string
	var includeProjectFiles bool
	var outputPath string
	var force bool

	command := &cobra.Command{
		Use:   "export NAME",
		Short: "Export an environment backup archive to a chosen path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := loadEnvironmentManifest(projectRoot, args[0])
			if err != nil {
				return err
			}

			if outputPath == "" {
				return fmt.Errorf("--output is required")
			}

			artifact, err := environment.NewBackupService(nil).Backup(manifest, environment.BackupCreateOptions{
				OutputPath:          outputPath,
				IncludeProjectFiles: includeProjectFiles,
				Force:               force,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Exported backup %s\n", artifact.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "Database: %s\n", artifact.Metadata.DatabaseName)
			fmt.Fprintf(cmd.OutOrStdout(), "Includes project files: %t\n", artifact.Metadata.IncludesProjectFiles)
			return nil
		},
	}

	command.Flags().StringVar(&projectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	command.Flags().StringVar(&outputPath, "output", "", "Output path for the exported backup archive")
	command.Flags().BoolVar(&includeProjectFiles, "include-project-files", false, "Include a project snapshot in the exported archive")
	command.Flags().BoolVar(&force, "force", false, "Overwrite the target output file if it already exists")

	return command
}

func newImportCommand() *cobra.Command {
	var projectRoot string
	var force bool

	command := &cobra.Command{
		Use:   "import NAME ARCHIVE",
		Short: "Copy a portable backup archive into an environment's managed backup store",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := loadEnvironmentManifest(projectRoot, args[0])
			if err != nil {
				return err
			}

			artifact, err := environment.NewBackupService(nil).Import(manifest, args[1], force)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Imported backup %s\n", artifact.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "Source environment: %s\n", artifact.Metadata.EnvironmentName)
			fmt.Fprintf(cmd.OutOrStdout(), "Includes project files: %t\n", artifact.Metadata.IncludesProjectFiles)
			return nil
		},
	}

	command.Flags().StringVar(&projectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	command.Flags().BoolVar(&force, "force", false, "Overwrite an imported archive with the same file name")

	return command
}

func newRestoreCommand() *cobra.Command {
	var projectRoot string
	var restoreProjectFiles bool
	var force bool

	command := &cobra.Command{
		Use:   "restore NAME ARCHIVE",
		Short: "Restore an environment database from a managed or portable backup archive",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := loadEnvironmentManifest(projectRoot, args[0])
			if err != nil {
				return err
			}

			result, err := environment.NewBackupService(nil).Restore(manifest, environment.RestoreOptions{
				ArchivePath:         args[1],
				RestoreProjectFiles: restoreProjectFiles,
				Force:               force,
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Restored backup %s\n", result.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "Database: %s\n", manifest.Runtime.Database.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Project files restored: %t\n", result.RestoredProjectFiles)
			return nil
		},
	}

	command.Flags().StringVar(&projectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	command.Flags().BoolVar(&restoreProjectFiles, "project-files", false, "Restore any project files included in the archive")
	command.Flags().BoolVar(&force, "force", false, "Confirm destructive database replacement during restore")

	return command
}

func loadEnvironmentManifest(projectRoot string, name string) (environment.Manifest, error) {
	manifestPath, err := environment.ResolveManifestPath(projectRoot, name)
	if err != nil {
		return environment.Manifest{}, err
	}

	return environment.LoadManifest(manifestPath)
}
