package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	home, _ := os.UserHomeDir()

	if len(cfg.ProjectDirs) == 0 {
		t.Fatal("DefaultConfig().ProjectDirs is empty, want home dir")
	}
	if cfg.ProjectDirs[0] != home {
		t.Errorf("ProjectDirs[0] = %q, want %q", cfg.ProjectDirs[0], home)
	}
}

func TestLoadFromNonexistent(t *testing.T) {
	cfg := LoadFrom("/nonexistent/path/to/config.json")
	def := DefaultConfig()

	if len(cfg.ProjectDirs) != len(def.ProjectDirs) {
		t.Errorf("LoadFrom(nonexistent) ProjectDirs len = %d, want %d (defaults)", len(cfg.ProjectDirs), len(def.ProjectDirs))
	}
}

func TestLoadFromValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{"project_dirs": ["/foo", "/bar"]}`
	os.WriteFile(path, []byte(data), 0644)

	cfg := LoadFrom(path)
	if len(cfg.ProjectDirs) != 2 {
		t.Fatalf("ProjectDirs len = %d, want 2", len(cfg.ProjectDirs))
	}
	if cfg.ProjectDirs[0] != "/foo" {
		t.Errorf("ProjectDirs[0] = %q, want /foo", cfg.ProjectDirs[0])
	}
	if cfg.ProjectDirs[1] != "/bar" {
		t.Errorf("ProjectDirs[1] = %q, want /bar", cfg.ProjectDirs[1])
	}
}

func TestLoadFromInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte("not valid json {{{"), 0644)

	cfg := LoadFrom(path)
	def := DefaultConfig()

	if len(cfg.ProjectDirs) != len(def.ProjectDirs) {
		t.Errorf("LoadFrom(invalid JSON) should return defaults, got ProjectDirs = %v", cfg.ProjectDirs)
	}
}

func TestGetProjectDirsExpandsTilde(t *testing.T) {
	cfg := Config{ProjectDirs: []string{"~/projects", "~/work"}}
	dirs := cfg.GetProjectDirs()
	home, _ := os.UserHomeDir()

	if len(dirs) != 2 {
		t.Fatalf("GetProjectDirs() len = %d, want 2", len(dirs))
	}
	want0 := filepath.Join(home, "projects")
	if dirs[0] != want0 {
		t.Errorf("dirs[0] = %q, want %q", dirs[0], want0)
	}
}

func TestGetProjectDirsAbsolutePaths(t *testing.T) {
	cfg := Config{ProjectDirs: []string{"/absolute/path"}}
	dirs := cfg.GetProjectDirs()
	if dirs[0] != "/absolute/path" {
		t.Errorf("dirs[0] = %q, want /absolute/path", dirs[0])
	}
}

func TestLoadFromSaveToRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := Config{ProjectDirs: []string{"/path/one", "/path/two"}}
	if err := SaveTo(path, original); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}

	loaded := LoadFrom(path)
	if len(loaded.ProjectDirs) != 2 {
		t.Fatalf("ProjectDirs len = %d, want 2", len(loaded.ProjectDirs))
	}
	if loaded.ProjectDirs[0] != "/path/one" {
		t.Errorf("ProjectDirs[0] = %q, want /path/one", loaded.ProjectDirs[0])
	}
	if loaded.ProjectDirs[1] != "/path/two" {
		t.Errorf("ProjectDirs[1] = %q, want /path/two", loaded.ProjectDirs[1])
	}
}

func TestSaveToCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "config.json")
	cfg := Config{ProjectDirs: []string{"/home/user"}}
	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo() error when directory missing: %v", err)
	}
}
