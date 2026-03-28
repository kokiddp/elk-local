package environment

import (
	"fmt"
	"strings"
)

type UpdateOptions struct {
	Name                 string
	ProjectRoot          string
	PHPVersion           string
	WebServer            string
	DatabaseEngine       string
	DatabaseVersion      string
	DatabaseName         string
	DatabaseUser         string
	DatabasePassword     string
	DatabaseRootPassword string
	EnableAdminer        bool
	DisableAdminer       bool
	AdminerPort          int
	EnableMailpit        bool
	DisableMailpit       bool
	MailpitHTTPPort      int
	MailpitSMTPPort      int
	EnableXdebug         bool
	DisableXdebug        bool
	XdebugHost           string
	XdebugPort           int
	HTTPPort             int
	DatabasePort         int
}

func Update(options UpdateOptions) (*CreatedEnvironment, error) {
	manifestPath, err := ResolveManifestPath(options.ProjectRoot, options.Name)
	if err != nil {
		return nil, err
	}

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	if options.EnableAdminer && options.DisableAdminer {
		return nil, fmt.Errorf("adminer cannot be enabled and disabled in the same command")
	}

	if options.EnableMailpit && options.DisableMailpit {
		return nil, fmt.Errorf("mailpit cannot be enabled and disabled in the same command")
	}

	if options.EnableXdebug && options.DisableXdebug {
		return nil, fmt.Errorf("xdebug cannot be enabled and disabled in the same command")
	}

	previousDatabaseEngine := manifest.Runtime.Database.Engine
	manifest.Runtime.PHPVersion = firstNonEmpty(options.PHPVersion, manifest.Runtime.PHPVersion)
	manifest.Runtime.WebServer = firstNonEmpty(options.WebServer, manifest.Runtime.WebServer)
	manifest.Runtime.Database.Engine = firstNonEmpty(options.DatabaseEngine, manifest.Runtime.Database.Engine)
	if strings.TrimSpace(options.DatabaseVersion) != "" {
		manifest.Runtime.Database.Version = options.DatabaseVersion
	} else if strings.TrimSpace(options.DatabaseEngine) != "" && !strings.EqualFold(previousDatabaseEngine, manifest.Runtime.Database.Engine) {
		manifest.Runtime.Database.Version = DefaultDatabaseVersion(manifest.Runtime.Database.Engine)
	}
	manifest.Runtime.Database.Name = firstNonEmpty(options.DatabaseName, manifest.Runtime.Database.Name)
	manifest.Runtime.Database.User = firstNonEmpty(options.DatabaseUser, manifest.Runtime.Database.User)
	manifest.Runtime.Database.Password = firstNonEmpty(options.DatabasePassword, manifest.Runtime.Database.Password)
	manifest.Runtime.Database.RootPassword = firstNonEmpty(options.DatabaseRootPassword, manifest.Runtime.Database.RootPassword)
	manifest.Network.HTTPPort = choosePort(options.HTTPPort, manifest.Network.HTTPPort)
	manifest.Network.DatabasePort = choosePort(options.DatabasePort, manifest.Network.DatabasePort)

	if options.EnableAdminer {
		manifest.Tooling.Adminer.Enabled = true
	}
	if options.DisableAdminer {
		manifest.Tooling.Adminer.Enabled = false
	}
	if options.AdminerPort > 0 {
		manifest.Tooling.Adminer.Port = options.AdminerPort
	}

	if options.EnableMailpit {
		manifest.Tooling.Mailpit.Enabled = true
	}
	if options.DisableMailpit {
		manifest.Tooling.Mailpit.Enabled = false
	}
	if options.MailpitHTTPPort > 0 {
		manifest.Tooling.Mailpit.HTTPPort = options.MailpitHTTPPort
	}
	if options.MailpitSMTPPort > 0 {
		manifest.Tooling.Mailpit.SMTPPort = options.MailpitSMTPPort
	}

	if options.EnableXdebug {
		manifest.Tooling.Xdebug.Enabled = true
	}
	if options.DisableXdebug {
		manifest.Tooling.Xdebug.Enabled = false
	}
	if strings.TrimSpace(options.XdebugHost) != "" {
		manifest.Tooling.Xdebug.ClientHost = options.XdebugHost
	}
	if options.XdebugPort > 0 {
		manifest.Tooling.Xdebug.ClientPort = options.XdebugPort
	}

	manifest.Runtime.WebServer = strings.ToLower(manifest.Runtime.WebServer)
	manifest.Runtime.Database.Engine = strings.ToLower(manifest.Runtime.Database.Engine)
	normalizeManifest(&manifest)

	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}

	return writeGeneratedArtifacts(manifest)
}
