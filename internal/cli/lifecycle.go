package cli

import (
	"fmt"
	"strings"

	"elk-local/internal/environment"

	"github.com/spf13/cobra"
)

func newStartCommand() *cobra.Command {
	return newLifecycleCommand(
		"start",
		"Start an environment with docker compose",
		func(service environment.LifecycleService, manifest environment.Manifest) (string, error) {
			return service.Start(manifest)
		},
	)
}

func newStopCommand() *cobra.Command {
	return newLifecycleCommand(
		"stop",
		"Stop an environment with docker compose",
		func(service environment.LifecycleService, manifest environment.Manifest) (string, error) {
			return service.Stop(manifest)
		},
	)
}

func newDestroyCommand() *cobra.Command {
	return newLifecycleCommand(
		"destroy",
		"Remove an environment's running containers with docker compose",
		func(service environment.LifecycleService, manifest environment.Manifest) (string, error) {
			return service.Destroy(manifest)
		},
	)
}

func newStatusCommand() *cobra.Command {
	return newLifecycleCommand(
		"status",
		"Show docker compose status for an environment",
		func(service environment.LifecycleService, manifest environment.Manifest) (string, error) {
			return service.Status(manifest)
		},
	)
}

func newLifecycleCommand(
	use string,
	short string,
	callback func(environment.LifecycleService, environment.Manifest) (string, error),
) *cobra.Command {
	var projectRoot string

	command := &cobra.Command{
		Use:   use + " NAME",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath, err := environment.ResolveManifestPath(projectRoot, args[0])
			if err != nil {
				return err
			}

			manifest, err := environment.LoadManifest(manifestPath)
			if err != nil {
				return err
			}

			service := environment.NewLifecycleService(nil)
			output, err := callback(service, manifest)
			if err != nil {
				if strings.TrimSpace(output) != "" {
					return fmt.Errorf("%w\n%s", err, output)
				}
				return err
			}

			if strings.TrimSpace(output) != "" {
				fmt.Fprintln(cmd.OutOrStdout(), output)
			}

			return nil
		},
	}

	command.Flags().StringVar(&projectRoot, "project-root", "", "Project root that contains .elk-local/environments")

	return command
}
