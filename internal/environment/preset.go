package environment

import (
	"fmt"
	"sort"
	"strings"
)

type Preset struct {
	Name              string
	Description       string
	ApplicationName   string
	DefaultAppVersion string
	AppVersionHint    string
	ProjectType       string
	PHPVersion        string
	WebServer         string
	DatabaseEngine    string
	DatabaseVersion   string
}

var presetCatalog = map[string]Preset{
	"wordpress": {
		Name:              "wordpress",
		Description:       "WordPress-oriented defaults with Apache and MariaDB.",
		ApplicationName:   "WordPress",
		DefaultAppVersion: "latest",
		AppVersionHint:    "Accepts latest, nightly, stable versions like 6.9.4, and prereleases like 7.0-beta6 or 7.0-RC2.",
		ProjectType:       "wordpress",
		PHPVersion:        "8.3",
		WebServer:         "apache",
		DatabaseEngine:    "mariadb",
		DatabaseVersion:   "11.4",
	},
	"laravel": {
		Name:              "laravel",
		Description:       "Laravel-style defaults with Nginx, PHP-FPM, and MariaDB.",
		ApplicationName:   "Laravel",
		DefaultAppVersion: "latest",
		AppVersionHint:    "Accepts latest or a Composer version constraint like ^12.0 or 11.*.",
		ProjectType:       "laravel",
		PHPVersion:        "8.3",
		WebServer:         "nginx",
		DatabaseEngine:    "mariadb",
		DatabaseVersion:   "11.4",
	},
	"symfony": {
		Name:              "symfony",
		Description:       "Symfony-style defaults with Nginx, PHP-FPM, and MySQL.",
		ApplicationName:   "Symfony",
		DefaultAppVersion: "latest",
		AppVersionHint:    "Accepts latest or a Composer version constraint like ^7.0 or 6.4.*.",
		ProjectType:       "symfony",
		PHPVersion:        "8.3",
		WebServer:         "nginx",
		DatabaseEngine:    "mysql",
		DatabaseVersion:   "8.4",
	},
	"custom": {
		Name:            "custom",
		Description:     "Framework-agnostic PHP defaults with Nginx and MariaDB.",
		ProjectType:     "php",
		PHPVersion:      "8.3",
		WebServer:       "nginx",
		DatabaseEngine:  "mariadb",
		DatabaseVersion: "11.4",
	},
}

func (preset Preset) InstallsApplication() bool {
	return strings.TrimSpace(preset.ApplicationName) != ""
}

func GetPreset(name string) (Preset, error) {
	preset, ok := presetCatalog[strings.ToLower(name)]
	if !ok {
		return Preset{}, fmt.Errorf("unknown preset %q", name)
	}

	return preset, nil
}

func ListPresets() []Preset {
	presets := make([]Preset, 0, len(presetCatalog))
	for _, preset := range presetCatalog {
		presets = append(presets, preset)
	}

	sort.Slice(presets, func(left int, right int) bool {
		return presets[left].Name < presets[right].Name
	})

	return presets
}

func DefaultDatabaseVersion(engine string) string {
	switch strings.ToLower(engine) {
	case "mysql":
		return "8.4"
	default:
		return "11.4"
	}
}
