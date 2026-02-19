package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// New hooks format: matcher is a regex string (omit to match everything).

type hookCommand struct {
	Type    string `json:"type"`    // always "command"
	Command string `json:"command"`
}

type hookRule struct {
	// Matcher is a regex string filtered against the tool/event name.
	// Omitting it (omitempty + zero value) matches all occurrences.
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

type hooksConfig struct {
	UserPromptSubmit []hookRule `json:"UserPromptSubmit"`
	PreToolUse       []hookRule `json:"PreToolUse"`
	PostToolUse      []hookRule `json:"PostToolUse"`
	Stop             []hookRule `json:"Stop"`
	Notification     []hookRule `json:"Notification"`
}

func makeRule(cmd string) hookRule {
	return hookRule{
		// No matcher — matches every occurrence of the event.
		Hooks: []hookCommand{{Type: "command", Command: cmd}},
	}
}

// Install writes the herd hooks into ~/.claude/settings.json.
// It preserves all existing keys.
func Install(herdBin string) error {
	settingsPath := claudeSettingsPath()

	// Read existing settings as raw JSON to preserve unknown fields.
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	hooks := hooksConfig{
		UserPromptSubmit: []hookRule{makeRule(herdBin + " hook UserPromptSubmit")},
		PreToolUse:       []hookRule{makeRule(herdBin + " hook PreToolUse")},
		PostToolUse:      []hookRule{makeRule(herdBin + " hook PostToolUse")},
		Stop:             []hookRule{makeRule(herdBin + " hook Stop")},
		Notification:     []hookRule{makeRule(herdBin + " hook Notification")},
	}

	hooksJSON, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	raw["hooks"] = json.RawMessage(hooksJSON)

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return os.WriteFile(settingsPath, data, 0o644)
}

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}
