package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds herd configuration.
type Config struct {
	// ProjectDirs is a list of directories to scan for projects.
	// Defaults to the user's home directory if empty.
	ProjectDirs []string `json:"project_dirs,omitempty"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		ProjectDirs: []string{home},
	}
}

// configPath returns the path to the config file.
func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".herd", "config.json")
}

// LoadFrom reads the config from the given path, or returns defaults if not found or invalid.
func LoadFrom(path string) Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	// Parse JSON, keeping defaults for missing fields
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return cfg
	}

	// Override defaults with loaded values
	if len(loaded.ProjectDirs) > 0 {
		cfg.ProjectDirs = loaded.ProjectDirs
	}

	return cfg
}

// SaveTo writes the config to the given path.
func SaveTo(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// Load reads the config from disk, or returns defaults if not found.
func Load() Config {
	return LoadFrom(configPath())
}

// Save writes the config to disk.
func Save(cfg Config) error {
	return SaveTo(configPath(), cfg)
}

// GetProjectDirs returns directories to scan for projects.
// Expands ~ to home directory.
func (c Config) GetProjectDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := make([]string, 0, len(c.ProjectDirs))

	for _, d := range c.ProjectDirs {
		// Expand ~ to home directory
		if len(d) > 0 && d[0] == '~' {
			d = filepath.Join(home, d[1:])
		}
		dirs = append(dirs, d)
	}

	return dirs
}
