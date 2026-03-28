package cli

import (
	"fmt"

	"elk-local/internal/environment"

	"github.com/spf13/cobra"
)

func newSwitchCommand() *cobra.Command {
	options := environment.UpdateOptions{}

	command := &cobra.Command{
		Use:   "switch NAME",
		Short: "Switch the runtime configuration of an existing environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Name = args[0]

			updated, err := environment.Update(options)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated environment %s\n", updated.Manifest.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "PHP: %s\n", updated.Manifest.Runtime.PHPVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "Web server: %s\n", updated.Manifest.Runtime.WebServer)
			fmt.Fprintf(cmd.OutOrStdout(), "Database: %s:%s\n", updated.Manifest.Runtime.Database.Engine, updated.Manifest.Runtime.Database.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "Database name: %s\n", updated.Manifest.Runtime.Database.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Database user: %s\n", updated.Manifest.Runtime.Database.User)
			fmt.Fprintf(cmd.OutOrStdout(), "Adminer: %t\n", updated.Manifest.Tooling.Adminer.Enabled)
			fmt.Fprintf(cmd.OutOrStdout(), "Mailpit: %t\n", updated.Manifest.Tooling.Mailpit.Enabled)
			fmt.Fprintf(cmd.OutOrStdout(), "Xdebug: %t\n", updated.Manifest.Tooling.Xdebug.Enabled)
			fmt.Fprintf(cmd.OutOrStdout(), "Compose: %s\n", updated.ComposePath)

			if updated.NginxConfigPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Nginx config: %s\n", updated.NginxConfigPath)
			}

			for _, path := range updated.AppConfigPaths {
				fmt.Fprintf(cmd.OutOrStdout(), "App config: %s\n", path)
			}

			return nil
		},
	}

	command.Flags().StringVar(&options.ProjectRoot, "project-root", "", "Project root that contains .elk-local/environments")
	command.Flags().StringVar(&options.PHPVersion, "php", "", "Switch to a PHP version from 7.4 through 8.4")
	command.Flags().StringVar(&options.WebServer, "web-server", "", "Switch to apache or nginx")
	command.Flags().StringVar(&options.DatabaseEngine, "database", "", "Switch to mysql or mariadb")
	command.Flags().StringVar(&options.DatabaseVersion, "database-version", "", "Override the database engine version")
	command.Flags().StringVar(&options.DatabaseName, "db-name", "", "Override the application database name")
	command.Flags().StringVar(&options.DatabaseUser, "db-user", "", "Override the application database user")
	command.Flags().StringVar(&options.DatabasePassword, "db-password", "", "Override the application database password")
	command.Flags().StringVar(&options.DatabaseRootPassword, "db-root-password", "", "Override the database root password")
	command.Flags().BoolVar(&options.EnableAdminer, "enable-adminer", false, "Enable Adminer for the environment")
	command.Flags().BoolVar(&options.DisableAdminer, "disable-adminer", false, "Disable Adminer for the environment")
	command.Flags().IntVar(&options.AdminerPort, "adminer-port", 0, "Override the Adminer host port")
	command.Flags().BoolVar(&options.EnableMailpit, "enable-mailpit", false, "Enable Mailpit for the environment")
	command.Flags().BoolVar(&options.DisableMailpit, "disable-mailpit", false, "Disable Mailpit for the environment")
	command.Flags().IntVar(&options.MailpitHTTPPort, "mailpit-port", 0, "Override the Mailpit HTTP host port")
	command.Flags().IntVar(&options.MailpitSMTPPort, "mailpit-smtp-port", 0, "Override the Mailpit SMTP host port")
	command.Flags().BoolVar(&options.EnableXdebug, "enable-xdebug", false, "Enable Xdebug for the environment")
	command.Flags().BoolVar(&options.DisableXdebug, "disable-xdebug", false, "Disable Xdebug for the environment")
	command.Flags().StringVar(&options.XdebugHost, "xdebug-client-host", "", "Override the Xdebug client host")
	command.Flags().IntVar(&options.XdebugPort, "xdebug-client-port", 0, "Override the Xdebug client port")
	command.Flags().IntVar(&options.HTTPPort, "http-port", 0, "Override the host HTTP port")
	command.Flags().IntVar(&options.DatabasePort, "db-port", 0, "Override the host database port")

	return command
}
