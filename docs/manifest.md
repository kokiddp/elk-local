# Environment Manifest

ELK-Local environments are defined by a human-readable YAML manifest plus generated artifacts such as a Compose file and, when needed, an Nginx config.

## Current Shape

```yaml
apiVersion: elk.dev/v1alpha1
kind: Environment
name: wp-demo
preset: wordpress
application:
  name: WordPress
  version: 6.9.4
project:
  type: wordpress
  root: /absolute/path/to/project
runtime:
  phpVersion: "8.3"
  webServer: apache
  database:
    engine: mariadb
    version: "11.4"
    name: wp_demo
    user: elk
    password: elk
    rootPassword: elkroot
tooling:
  adminer:
    enabled: true
    port: 45036
  mailpit:
    enabled: true
    httpPort: 50036
    smtpPort: 60036
  xdebug:
    enabled: true
    clientHost: host.docker.internal
    clientPort: 9003
    mode: debug,develop
    ideKey: VSCODE
network:
  httpPort: 25036
  databasePort: 35036
storage:
  basePath: /absolute/path/to/project/.elk-local/environments/wp-demo
  backupsPath: /absolute/path/to/project/.elk-local/environments/wp-demo/backups
compose:
  projectName: elk-wp-demo
  namePrefix: elk-wp-demo
  file: /absolute/path/to/project/.elk-local/environments/wp-demo/compose.yaml
```

## Notes

- Ports are deterministic by environment name unless overridden.
- Container and volume names are derived from the environment name with the `elk-<environment>` prefix.
- The output directory defaults to `.elk-local/environments/<name>` under the project root.
- `application.name` and `application.version` track what ELK-Local installed for presets that bootstrap a real application.
- Database name, user, password, and root password are part of the manifest and drive both container configuration and application config syncing.
- Adminer, Mailpit, and Xdebug are optional and disabled by default.
- Nginx-based presets also generate `nginx/default.conf` alongside the Compose file.
- Adminer-enabled environments also generate an `adminer/` directory with a managed image that logs directly into the configured database.
- Xdebug-enabled environments also generate an `xdebug/` directory with a Dockerfile and PHP config.
- `storage.backupsPath` is the default destination for managed backup archives and imported portable archives.
- Presets that install an application expect an empty project root or an already compatible application tree.
- The manifest is the source of truth. Generated files should be treated as build artifacts.
- Existing manifests can be regenerated with `elk-local switch <name>` to change PHP, web server, database settings, or database credentials.

## Built-In Presets

- `wordpress`: Apache + MariaDB
- `laravel`: Nginx + MariaDB
- `symfony`: Nginx + MySQL
- `custom`: Nginx + MariaDB
