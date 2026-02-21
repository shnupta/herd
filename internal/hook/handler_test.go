package hook

import (
	"fmt"
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

func TestProcessInvalidJSON(t *testing.T) {
	err := process("UserPromptSubmit", strings.NewReader(`{not valid json}`), func(s state.SessionState) error {
		t.Fatal("write should not be called for invalid JSON")
		return nil
	})
	if err == nil {
		t.Fatal("process() should return error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode stdin") {
		t.Errorf("error = %q, want it to contain 'decode stdin'", err.Error())
	}
}

func TestProcessWriteError(t *testing.T) {
	writeErr := fmt.Errorf("disk full")
	err := process("Stop", strings.NewReader(makeInput("sess-err", "")), func(s state.SessionState) error {
		return writeErr
	})
	if err == nil {
		t.Fatal("process() should propagate write error")
	}
	if err != writeErr {
		t.Errorf("error = %v, want %v", err, writeErr)
	}
}

func TestProcessSetsProjectPath(t *testing.T) {
	got := captureWrite(t, "UserPromptSubmit", makeInput("sess-cwd", ""))
	if got.ProjectPath == "" {
		t.Error("ProjectPath should be set to current working directory")
	}
}

func TestProcessSetsUpdatedAt(t *testing.T) {
	got := captureWrite(t, "Stop", makeInput("sess-time", ""))
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestProcessEmptyReader(t *testing.T) {
	err := process("Stop", strings.NewReader(""), func(s state.SessionState) error {
		t.Fatal("write should not be called for empty input")
		return nil
	})
	if err == nil {
		t.Fatal("process() should return error for empty input")
	}
}

func TestProcessToolInputPassthrough(t *testing.T) {
	input := `{"session_id":"sess-extra","tool_name":"Write","tool_input":{"file":"/tmp/x"},"message":"test"}`
	got := captureWrite(t, "PreToolUse", input)
	if got.SessionID != "sess-extra" {
		t.Errorf("SessionID = %q, want sess-extra", got.SessionID)
	}
	if got.CurrentTool != "Write" {
		t.Errorf("CurrentTool = %q, want Write", got.CurrentTool)
	}
}
