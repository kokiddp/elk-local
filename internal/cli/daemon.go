package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"elk-local/internal/config"
	"elk-local/internal/daemon"

	"github.com/spf13/cobra"
)

const (
	defaultDaemonHost = "127.0.0.1"
)

func newDaemonCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the background ELK-Local API daemon",
	}

	command.AddCommand(newDaemonStartCommand())
	command.AddCommand(newDaemonStopCommand())
	command.AddCommand(newDaemonStatusCommand())
	command.AddCommand(newDaemonRestartCommand())

	return command
}

func newDaemonStartCommand() *cobra.Command {
	options := daemon.StartOptions{}

	command := &cobra.Command{
		Use:   "start",
		Short: "Start the ELK-Local API daemon in the background",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := daemon.Start(options)
			if err != nil {
				return err
			}

			printDaemonRunning(cmd, "ELK-Local daemon started.", status)
			return nil
		},
	}

	addDaemonServeFlags(command, &options.Host, &options.Port, &options.ProjectRoot, &options.WebUIDist)
	return command
}

func newDaemonStopCommand() *cobra.Command {
	var projectRoot string

	command := &cobra.Command{
		Use:   "stop",
		Short: "Stop the background ELK-Local API daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := daemon.Stop(projectRoot)
			if err != nil {
				if errors.Is(err, daemon.ErrNotRunning) {
					fmt.Fprintln(cmd.OutOrStdout(), "ELK-Local daemon is not running.")
					return nil
				}
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "ELK-Local daemon stopped.")
			fmt.Fprintf(cmd.OutOrStdout(), "Project root: %s\n", status.State.ProjectRoot)
			return nil
		},
	}

	command.Flags().StringVar(&projectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	return command
}

func newDaemonStatusCommand() *cobra.Command {
	var projectRoot string

	command := &cobra.Command{
		Use:   "status",
		Short: "Show ELK-Local daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := daemon.Inspect(projectRoot)
			if err != nil {
				return err
			}

			if status.State.PID == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "ELK-Local daemon is not running.")
				return nil
			}

			if status.Running {
				printDaemonRunning(cmd, "ELK-Local daemon is running.", status)
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "ELK-Local daemon state exists but the server is not reachable.")
			fmt.Fprintf(cmd.OutOrStdout(), "Project root: %s\n", status.State.ProjectRoot)
			fmt.Fprintf(cmd.OutOrStdout(), "PID: %d\n", status.State.PID)
			fmt.Fprintf(cmd.OutOrStdout(), "Expected URL: %s\n", status.State.BaseURL())
			fmt.Fprintf(cmd.OutOrStdout(), "Log file: %s\n", status.State.LogFile)
			return fmt.Errorf("daemon is not reachable")
		},
	}

	command.Flags().StringVar(&projectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	return command
}

func newDaemonRestartCommand() *cobra.Command {
	options := daemon.StartOptions{}

	command := &cobra.Command{
		Use:   "restart",
		Short: "Restart the background ELK-Local API daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := daemon.Restart(options)
			if err != nil {
				return err
			}

			printDaemonRunning(cmd, "ELK-Local daemon restarted.", status)
			return nil
		},
	}

	addDaemonServeFlags(command, &options.Host, &options.Port, &options.ProjectRoot, &options.WebUIDist)
	return command
}

func addDaemonServeFlags(command *cobra.Command, host *string, port *int, projectRoot *string, webuiDist *string) {
	defaultWebUIDist := filepath.Join("webui", "dist")
	defaultDaemonPort, err := config.ResolveWebUIPort(0)
	if err != nil {
		defaultDaemonPort = config.DefaultWebUIPort
	}
	command.Flags().StringVar(host, "host", defaultDaemonHost, "Host interface to bind the local API daemon to")
	command.Flags().IntVar(port, "port", defaultDaemonPort, "Port to bind the local API daemon to")
	command.Flags().StringVar(projectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	command.Flags().StringVar(webuiDist, "webui-dist", defaultWebUIDist, "Optional built web UI directory to serve alongside the API")
}

func printDaemonRunning(cmd *cobra.Command, headline string, status daemon.Status) {
	fmt.Fprintln(cmd.OutOrStdout(), headline)
	fmt.Fprintf(cmd.OutOrStdout(), "URL: %s\n", status.State.BaseURL())
	fmt.Fprintf(cmd.OutOrStdout(), "PID: %d\n", status.State.PID)
	fmt.Fprintf(cmd.OutOrStdout(), "Project root: %s\n", status.State.ProjectRoot)
	if status.Remote.StaticDir != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Web UI assets: %s\n", status.Remote.StaticDir)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Log file: %s\n", status.State.LogFile)
}
