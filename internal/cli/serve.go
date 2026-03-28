package cli

import (
	"fmt"
	"path/filepath"

	"elk-local/internal/api"
	"elk-local/internal/config"

	"github.com/spf13/cobra"
)

func newServeCommand() *cobra.Command {
	var host string
	var port int
	var projectRoot string
	var webuiDist string

	command := &cobra.Command{
		Use:   "serve",
		Short: "Start the local API in the foreground",
		Long:  "Start the local API in the foreground. Use `elk-local daemon start` for the background control plane.",
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := api.NewServer(projectRoot, webuiDist, nil, nil)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "ELK-Local API listening on http://%s:%d\n", host, port)
			if serverPath := resolvedWebUIDist(server); serverPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Serving web UI assets from %s\n", serverPath)
			}

			return server.ListenAndServe(host, port)
		},
	}

	defaultWebUIDist := filepath.Join("webui", "dist")
	defaultWebUIPort, err := config.ResolveWebUIPort(0)
	if err != nil {
		defaultWebUIPort = config.DefaultWebUIPort
	}
	command.Flags().StringVar(&host, "host", "127.0.0.1", "Host interface to bind the local API to")
	command.Flags().IntVar(&port, "port", defaultWebUIPort, "Port to bind the local API to")
	command.Flags().StringVar(&projectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	command.Flags().StringVar(&webuiDist, "webui-dist", defaultWebUIDist, "Optional built web UI directory to serve alongside the API")

	return command
}

func resolvedWebUIDist(server *api.Server) string {
	if server == nil {
		return ""
	}

	return server.StaticDir()
}
