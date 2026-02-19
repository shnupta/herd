package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/shnupta/herd/internal/state"
)

// hookInput is the JSON Claude Code sends to hook commands via stdin.
type hookInput struct {
	SessionID   string          `json:"session_id"`
	ToolName    string          `json:"tool_name"`
	ToolInput   json.RawMessage `json:"tool_input"`
	Message     string          `json:"message"` // for Notification
}

// Run processes a hook event. eventType is one of:
// "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop", "Notification".
func Run(eventType string) error {
	var input hookInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return fmt.Errorf("decode stdin: %w", err)
	}

	if input.SessionID == "" {
		return nil // nothing to track
	}

	s := state.SessionState{
		SessionID:   input.SessionID,
		TmuxPane:    os.Getenv("TMUX_PANE"),
		CurrentTool: input.ToolName,
		ProjectPath: cwd(),
		UpdatedAt:   time.Now(),
	}

	switch eventType {
	case "UserPromptSubmit":
		s.State = "working"
		s.CurrentTool = ""
	case "PreToolUse":
		if input.ToolName == "ExitPlanMode" {
			s.State = "plan_ready"
		} else {
			s.State = "working"
		}
	case "PostToolUse":
		s.State = "working" // still processing, next PreToolUse or Stop will follow
	case "Stop":
		s.State = "waiting"
		s.CurrentTool = ""
	case "Notification":
		s.State = "notifying"
	default:
		s.State = "unknown"
	}

	return state.Write(s)
}

func cwd() string {
	dir, _ := os.Getwd()
	return dir
}
