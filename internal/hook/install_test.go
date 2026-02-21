package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCreatesSettingsFromScratch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Install("/usr/local/bin/herd"); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("settings.json not valid JSON: %v", err)
	}

	if _, ok := raw["hooks"]; !ok {
		t.Fatal("settings.json missing hooks key")
	}

	var hooks hooksConfig
	if err := json.Unmarshal(raw["hooks"], &hooks); err != nil {
		t.Fatalf("hooks not valid: %v", err)
	}
	if len(hooks.Stop) != 1 {
		t.Fatalf("expected 1 Stop hook rule, got %d", len(hooks.Stop))
	}
	if hooks.Stop[0].Hooks[0].Command != "/usr/local/bin/herd hook Stop" {
		t.Fatalf("unexpected Stop command: %s", hooks.Stop[0].Hooks[0].Command)
	}
}

func TestInstallPreservesExistingKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create existing settings with a custom key
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := map[string]interface{}{
		"customKey": "customValue",
		"nested":    map[string]interface{}{"a": 1},
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install("/bin/herd"); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	result, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(result, &raw); err != nil {
		t.Fatal(err)
	}

	// Check that existing keys are preserved
	if _, ok := raw["customKey"]; !ok {
		t.Fatal("customKey was not preserved")
	}
	if _, ok := raw["nested"]; !ok {
		t.Fatal("nested key was not preserved")
	}
	// Check that hooks were added
	if _, ok := raw["hooks"]; !ok {
		t.Fatal("hooks key was not added")
	}
}

func TestInstallOverwritesExistingHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create settings with old hooks
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"Stop": []interface{}{
				map[string]interface{}{
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": "old-binary hook Stop"},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install("/new/herd"); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	result, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(result, &raw); err != nil {
		t.Fatal(err)
	}

	var hooks hooksConfig
	if err := json.Unmarshal(raw["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}

	// Verify hooks were replaced with new binary path
	if hooks.Stop[0].Hooks[0].Command != "/new/herd hook Stop" {
		t.Fatalf("hooks not updated: %s", hooks.Stop[0].Hooks[0].Command)
	}
}

func TestInstallWithMalformedExistingJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write malformed JSON — Install should overwrite gracefully
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{broken`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install("/bin/herd"); err != nil {
		t.Fatalf("Install() should succeed with malformed existing JSON: %v", err)
	}

	// Verify valid JSON was written
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := raw["hooks"]; !ok {
		t.Fatal("hooks key missing after install over malformed file")
	}
}

func TestInstallAllHookEvents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Install("/bin/herd"); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	var hooks hooksConfig
	if err := json.Unmarshal(raw["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}

	events := map[string][]hookRule{
		"UserPromptSubmit": hooks.UserPromptSubmit,
		"PreToolUse":       hooks.PreToolUse,
		"PostToolUse":      hooks.PostToolUse,
		"Stop":             hooks.Stop,
		"Notification":     hooks.Notification,
	}

	for name, rules := range events {
		if len(rules) != 1 {
			t.Errorf("expected 1 rule for %s, got %d", name, len(rules))
			continue
		}
		if len(rules[0].Hooks) != 1 {
			t.Errorf("expected 1 hook for %s, got %d", name, len(rules[0].Hooks))
			continue
		}
		expected := "/bin/herd hook " + name
		if rules[0].Hooks[0].Command != expected {
			t.Errorf("%s command = %q, want %q", name, rules[0].Hooks[0].Command, expected)
		}
		if rules[0].Hooks[0].Type != "command" {
			t.Errorf("%s type = %q, want command", name, rules[0].Hooks[0].Type)
		}
		if rules[0].Matcher != "" {
			t.Errorf("%s matcher = %q, want empty", name, rules[0].Matcher)
		}
	}
}

func TestMakeRule(t *testing.T) {
	rule := makeRule("herd hook Stop")
	if len(rule.Hooks) != 1 {
		t.Fatalf("Hooks count = %d, want 1", len(rule.Hooks))
	}
	if rule.Hooks[0].Type != "command" {
		t.Errorf("Type = %q, want command", rule.Hooks[0].Type)
	}
	if rule.Hooks[0].Command != "herd hook Stop" {
		t.Errorf("Command = %q, want 'herd hook Stop'", rule.Hooks[0].Command)
	}
	if rule.Matcher != "" {
		t.Errorf("Matcher = %q, want empty", rule.Matcher)
	}
}
