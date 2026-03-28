package cli

import (
	"fmt"

	"elk-local/internal/environment"

	"github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
	options := environment.CreateOptions{}

	command := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a managed local environment scaffold",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Name = args[0]

			created, err := environment.Create(options)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created environment %s\n", created.Manifest.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Preset: %s\n", created.Manifest.Preset)
			fmt.Fprintf(cmd.OutOrStdout(), "Manifest: %s\n", created.ManifestPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Compose: %s\n", created.ComposePath)
			if created.Manifest.Application.Name != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Application: %s\n", created.Manifest.Application.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "Application version: %s\n", created.Manifest.Application.Version)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "HTTP: http://127.0.0.1:%d\n", created.Manifest.Network.HTTPPort)
			fmt.Fprintf(cmd.OutOrStdout(), "Database port: %d\n", created.Manifest.Network.DatabasePort)
			fmt.Fprintf(cmd.OutOrStdout(), "Database name: %s\n", created.Manifest.Runtime.Database.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Database user: %s\n", created.Manifest.Runtime.Database.User)

			if created.Manifest.Tooling.Adminer.Enabled {
				fmt.Fprintf(cmd.OutOrStdout(), "Adminer: http://127.0.0.1:%d\n", created.Manifest.Tooling.Adminer.Port)
			}

			if created.Manifest.Tooling.Mailpit.Enabled {
				fmt.Fprintf(cmd.OutOrStdout(), "Mailpit UI: http://127.0.0.1:%d\n", created.Manifest.Tooling.Mailpit.HTTPPort)
				fmt.Fprintf(cmd.OutOrStdout(), "Mailpit SMTP: 127.0.0.1:%d\n", created.Manifest.Tooling.Mailpit.SMTPPort)
			}

			if created.Manifest.Tooling.Xdebug.Enabled {
				fmt.Fprintf(cmd.OutOrStdout(), "Xdebug: %s:%d\n", created.Manifest.Tooling.Xdebug.ClientHost, created.Manifest.Tooling.Xdebug.ClientPort)
			}

			if created.NginxConfigPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Nginx config: %s\n", created.NginxConfigPath)
			}

			if created.XdebugDirPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Xdebug files: %s\n", created.XdebugDirPath)
			}

			for _, path := range created.AppConfigPaths {
				fmt.Fprintf(cmd.OutOrStdout(), "App config: %s\n", path)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Next: docker compose -f %s up -d\n", created.ComposePath)

			return nil
		},
	}

	command.Flags().StringVar(&options.Preset, "preset", "wordpress", "Preset to use: wordpress, laravel, symfony, custom")
	command.Flags().StringVar(&options.ApplicationVersion, "app-version", "", "Application version to install for the selected preset")
	command.Flags().StringVar(&options.ProjectRoot, "project-root", "", "Application project root to mount into the containers")
	command.Flags().StringVar(&options.OutputDir, "output", "", "Directory where ELK-Local should write the environment files")
	command.Flags().StringVar(&options.PHPVersion, "php", "", "Override the PHP version, for example 8.3")
	command.Flags().StringVar(&options.WebServer, "web-server", "", "Override the web server, either apache or nginx")
	command.Flags().StringVar(&options.DatabaseEngine, "database", "", "Override the database engine, either mysql or mariadb")
	command.Flags().StringVar(&options.DatabaseVersion, "database-version", "", "Override the database engine version")
	command.Flags().StringVar(&options.DatabaseName, "db-name", "", "Override the application database name")
	command.Flags().StringVar(&options.DatabaseUser, "db-user", "", "Override the application database user")
	command.Flags().StringVar(&options.DatabasePassword, "db-password", "", "Override the application database password")
	command.Flags().StringVar(&options.DatabaseRootPassword, "db-root-password", "", "Override the database root password")
	command.Flags().BoolVar(&options.AdminerEnabled, "adminer", false, "Include Adminer in the generated environment")
	command.Flags().IntVar(&options.AdminerPort, "adminer-port", 0, "Override the Adminer host port")
	command.Flags().BoolVar(&options.MailpitEnabled, "mailpit", false, "Include Mailpit in the generated environment")
	command.Flags().IntVar(&options.MailpitHTTPPort, "mailpit-port", 0, "Override the Mailpit HTTP host port")
	command.Flags().IntVar(&options.MailpitSMTPPort, "mailpit-smtp-port", 0, "Override the Mailpit SMTP host port")
	command.Flags().BoolVar(&options.XdebugEnabled, "xdebug", false, "Build the PHP image with Xdebug enabled")
	command.Flags().StringVar(&options.XdebugHost, "xdebug-client-host", "", "Override the Xdebug client host")
	command.Flags().IntVar(&options.XdebugPort, "xdebug-client-port", 0, "Override the Xdebug client port")
	command.Flags().IntVar(&options.HTTPPort, "http-port", 0, "Override the host HTTP port")
	command.Flags().IntVar(&options.DatabasePort, "db-port", 0, "Override the host database port")
	command.Flags().BoolVar(&options.Force, "force", false, "Overwrite previously generated files in the target directory")

	return command
}
