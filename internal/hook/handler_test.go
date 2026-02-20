package hook

import (
	"strings"
	"testing"

	"github.com/shnupta/herd/internal/state"
)

func makeInput(sessionID, toolName string) string {
	if toolName != "" {
		return `{"session_id":"` + sessionID + `","tool_name":"` + toolName + `"}`
	}
	return `{"session_id":"` + sessionID + `"}`
}

func captureWrite(t *testing.T, eventType, input string) state.SessionState {
	t.Helper()
	var got state.SessionState
	called := false
	err := process(eventType, strings.NewReader(input), func(s state.SessionState) error {
		got = s
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("process(%q) error: %v", eventType, err)
	}
	if !called {
		t.Fatalf("write function was not called for event %q", eventType)
	}
	return got
}

func TestProcessUserPromptSubmit(t *testing.T) {
	got := captureWrite(t, "UserPromptSubmit", makeInput("sess-1", "SomeTool"))
	if got.State != "working" {
		t.Errorf("State = %q, want working", got.State)
	}
	if got.CurrentTool != "" {
		t.Errorf("CurrentTool = %q, want empty (cleared for UserPromptSubmit)", got.CurrentTool)
	}
	if got.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", got.SessionID)
	}
}

func TestProcessPreToolUseRegular(t *testing.T) {
	got := captureWrite(t, "PreToolUse", makeInput("sess-2", "Bash"))
	if got.State != "working" {
		t.Errorf("State = %q, want working", got.State)
	}
	if got.CurrentTool != "Bash" {
		t.Errorf("CurrentTool = %q, want Bash", got.CurrentTool)
	}
}

func TestProcessPreToolUseExitPlanMode(t *testing.T) {
	got := captureWrite(t, "PreToolUse", makeInput("sess-3", "ExitPlanMode"))
	if got.State != "plan_ready" {
		t.Errorf("State = %q, want plan_ready", got.State)
	}
}

func TestProcessPostToolUse(t *testing.T) {
	got := captureWrite(t, "PostToolUse", makeInput("sess-4", "Read"))
	if got.State != "working" {
		t.Errorf("State = %q, want working", got.State)
	}
}

func TestProcessStop(t *testing.T) {
	got := captureWrite(t, "Stop", makeInput("sess-5", ""))
	if got.State != "waiting" {
		t.Errorf("State = %q, want waiting", got.State)
	}
	if got.CurrentTool != "" {
		t.Errorf("CurrentTool = %q, want empty (cleared for Stop)", got.CurrentTool)
	}
}

func TestProcessNotification(t *testing.T) {
	got := captureWrite(t, "Notification", `{"session_id":"sess-6","message":"hey"}`)
	if got.State != "notifying" {
		t.Errorf("State = %q, want notifying", got.State)
	}
}

func TestProcessUnknownEventType(t *testing.T) {
	got := captureWrite(t, "FutureThing", makeInput("sess-7", ""))
	if got.State != "unknown" {
		t.Errorf("State = %q, want unknown", got.State)
	}
}

func TestProcessEmptySessionIDIsNoOp(t *testing.T) {
	called := false
	err := process("UserPromptSubmit", strings.NewReader(`{"session_id":""}`), func(s state.SessionState) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("process() error: %v", err)
	}
	if called {
		t.Error("write function should not be called when session_id is empty")
	}
}
