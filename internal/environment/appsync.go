package environment

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type envSetting struct {
	Key   string
	Value string
}

var envKeyPattern = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=`)

func syncApplicationConfig(manifest Manifest) ([]string, error) {
	switch strings.ToLower(manifest.Project.Type) {
	case "wordpress":
		path, err := syncWordPressConfig(manifest)
		if err != nil {
			return nil, err
		}

		return []string{path}, nil
	case "laravel":
		path, err := syncDotEnvFile(filepath.Join(manifest.Project.Root, ".env"), laravelEnvSettings(manifest))
		if err != nil {
			return nil, err
		}

		return []string{path}, nil
	case "symfony":
		path, err := syncDotEnvFile(filepath.Join(manifest.Project.Root, ".env.local"), symfonyEnvSettings(manifest))
		if err != nil {
			return nil, err
		}

		return []string{path}, nil
	default:
		path, err := syncDotEnvFile(resolveGenericEnvPath(manifest.Project.Root), genericEnvSettings(manifest))
		if err != nil {
			return nil, err
		}

		return []string{path}, nil
	}
}

func resolveGenericEnvPath(projectRoot string) string {
	localPath := filepath.Join(projectRoot, ".env.local")
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	envPath := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envPath); err == nil {
		return envPath
	}

	return localPath
}

func laravelEnvSettings(manifest Manifest) []envSetting {
	return []envSetting{
		{Key: "DB_CONNECTION", Value: applicationDatabaseDriver(manifest)},
		{Key: "DB_HOST", Value: databaseServiceHost()},
		{Key: "DB_PORT", Value: fmt.Sprintf("%d", databaseServicePort())},
		{Key: "DB_DATABASE", Value: manifest.Runtime.Database.Name},
		{Key: "DB_USERNAME", Value: manifest.Runtime.Database.User},
		{Key: "DB_PASSWORD", Value: manifest.Runtime.Database.Password},
	}
}

func symfonyEnvSettings(manifest Manifest) []envSetting {
	return []envSetting{{
		Key:   "DATABASE_URL",
		Value: applicationDatabaseURL(manifest),
	}}
}

func genericEnvSettings(manifest Manifest) []envSetting {
	return []envSetting{
		{Key: "DB_CONNECTION", Value: applicationDatabaseDriver(manifest)},
		{Key: "DB_HOST", Value: databaseServiceHost()},
		{Key: "DB_PORT", Value: fmt.Sprintf("%d", databaseServicePort())},
		{Key: "DB_DATABASE", Value: manifest.Runtime.Database.Name},
		{Key: "DB_USERNAME", Value: manifest.Runtime.Database.User},
		{Key: "DB_PASSWORD", Value: manifest.Runtime.Database.Password},
		{Key: "DATABASE_URL", Value: applicationDatabaseURL(manifest)},
	}
}

func applicationDatabaseDriver(manifest Manifest) string {
	switch strings.ToLower(manifest.Runtime.Database.Engine) {
	case "mysql", "mariadb":
		return "mysql"
	default:
		return manifest.Runtime.Database.Engine
	}
}

func applicationDatabaseURL(manifest Manifest) string {
	connectionURL := &url.URL{
		Scheme: applicationDatabaseDriver(manifest),
		User:   url.UserPassword(manifest.Runtime.Database.User, manifest.Runtime.Database.Password),
		Host:   fmt.Sprintf("%s:%d", databaseServiceHost(), databaseServicePort()),
		Path:   "/" + manifest.Runtime.Database.Name,
	}

	return connectionURL.String()
}

func databaseServiceHost() string {
	return "db"
}

func databaseServicePort() int {
	return 3306
}

func syncDotEnvFile(path string, settings []envSetting) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read env file: %w", err)
	}

	lines := []string{}
	if len(contents) > 0 {
		lines = strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n")
	}

	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	found := map[string]bool{}
	for index, line := range lines {
		key := envKeyForLine(line)
		if key == "" {
			continue
		}

		for _, setting := range settings {
			if key != setting.Key {
				continue
			}

			lines[index] = fmt.Sprintf("%s=%s", setting.Key, quoteEnvValue(setting.Value))
			found[setting.Key] = true
			break
		}
	}

	if len(lines) == 0 {
		lines = append(lines, "# Managed by ELK-Local")
	}

	for _, setting := range settings {
		if found[setting.Key] {
			continue
		}

		lines = append(lines, fmt.Sprintf("%s=%s", setting.Key, quoteEnvValue(setting.Value)))
	}

	content := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write env file: %w", err)
	}

	return path, nil
}

func envKeyForLine(line string) string {
	matches := envKeyPattern.FindStringSubmatch(line)
	if len(matches) != 2 {
		return ""
	}

	return matches[1]
}

func quoteEnvValue(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return fmt.Sprintf(`"%s"`, escaped)
}

func syncWordPressConfig(manifest Manifest) (string, error) {
	configPath := filepath.Join(manifest.Project.Root, "wp-config.php")
	samplePath := filepath.Join(manifest.Project.Root, "wp-config-sample.php")

	contents, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			contents, err = os.ReadFile(samplePath)
			if err != nil {
				if os.IsNotExist(err) {
					contents = []byte("<?php\n")
				} else {
					return "", fmt.Errorf("read wp-config sample: %w", err)
				}
			}
		} else {
			return "", fmt.Errorf("read wp-config: %w", err)
		}
	}

	configText := strings.ReplaceAll(string(contents), "\r\n", "\n")
	configText = upsertWordPressDefine(configText, "DB_NAME", manifest.Runtime.Database.Name)
	configText = upsertWordPressDefine(configText, "DB_USER", manifest.Runtime.Database.User)
	configText = upsertWordPressDefine(configText, "DB_PASSWORD", manifest.Runtime.Database.Password)
	configText = upsertWordPressDefine(configText, "DB_HOST", databaseServiceHost())
	configText = upsertWordPressDefine(configText, "WP_HOME", wordPressSiteURL(manifest))
	configText = upsertWordPressDefine(configText, "WP_SITEURL", wordPressSiteURL(manifest))
	configText = upsertWordPressDefine(configText, "FS_METHOD", "direct")

	if !strings.Contains(configText, "wp-settings.php") {
		configText = strings.TrimRight(configText, "\n") + "\n\nif (!defined('ABSPATH')) {\n    define('ABSPATH', __DIR__ . '/');\n}\n\nrequire_once ABSPATH . 'wp-settings.php';\n"
	}

	if err := os.WriteFile(configPath, []byte(configText), 0o644); err != nil {
		return "", fmt.Errorf("write wp-config: %w", err)
	}

	return configPath, nil
}

func wordPressSiteURL(manifest Manifest) string {
	return fmt.Sprintf("http://127.0.0.1:%d", manifest.Network.HTTPPort)
}

func upsertWordPressDefine(contents string, constantName string, value string) string {
	pattern := regexp.MustCompile(`(?m)^(\s*define\(\s*['"]` + regexp.QuoteMeta(constantName) + `['"]\s*,\s*)(.+?)(\s*\)\s*;)`)
	quotedValue := phpSingleQuoted(value)
	location := pattern.FindStringSubmatchIndex(contents)
	if location != nil {
		prefix := contents[location[2]:location[3]]
		suffix := contents[location[6]:location[7]]
		return contents[:location[0]] + prefix + quotedValue + suffix + contents[location[1]:]
	}

	line := fmt.Sprintf("define('%s', %s);\n", constantName, quotedValue)
	if strings.HasPrefix(strings.TrimSpace(contents), "<?php") {
		parts := strings.SplitN(contents, "\n", 2)
		if len(parts) == 1 {
			return parts[0] + "\n" + line
		}

		return parts[0] + "\n" + line + parts[1]
	}

	return "<?php\n" + line + contents
}

func phpSingleQuoted(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}
