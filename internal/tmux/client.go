package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Pane represents a tmux pane with its metadata.
type Pane struct {
	ID          string // e.g. "%12"
	SessionID   string // e.g. "$2"
	SessionName string // e.g. "2"
	WindowIndex int
	PaneIndex   int
	PID         int
	CurrentCmd  string
	CurrentPath string
	Width       int
	Height      int
}

const listFormat = "#{pane_id}\t#{session_id}\t#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_pid}\t#{pane_current_command}\t#{pane_current_path}\t#{pane_width}\t#{pane_height}"

// ListPanes returns all panes across all tmux sessions.
func ListPanes() ([]Pane, error) {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F", listFormat).Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}

	var panes []Pane
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 10 {
			continue
		}
		pid, _ := strconv.Atoi(f[5])
		wIdx, _ := strconv.Atoi(f[3])
		pIdx, _ := strconv.Atoi(f[4])
		w, _ := strconv.Atoi(f[8])
		h, _ := strconv.Atoi(f[9])
		panes = append(panes, Pane{
			ID:          f[0],
			SessionID:   f[1],
			SessionName: f[2],
			WindowIndex: wIdx,
			PaneIndex:   pIdx,
			PID:         pid,
			CurrentCmd:  f[6],
			CurrentPath: f[7],
			Width:       w,
			Height:      h,
		})
	}
	return panes, nil
}

// IsClaudePane returns true if a pane's current command looks like Claude.
// Claude Code names its foreground process after its version (e.g. "2.1.47"),
// and tmux reports this via pane_current_command. We also accept "claude"
// for forward compatibility.
func IsClaudePane(currentCmd string) bool {
	if strings.EqualFold(currentCmd, "claude") {
		return true
	}
	return isVersionString(currentCmd)
}

func isVersionString(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

// CapturePane returns the contents of a pane with ANSI SGR codes preserved.
// tmux strips cursor-movement codes, so the output is safe to embed in a viewport.
func CapturePane(paneID string, scrollbackLines int) (string, error) {
	out, err := exec.Command(
		"tmux", "capture-pane",
		"-p",                                    // print to stdout
		"-e",                                    // preserve SGR escape codes
		"-t", paneID,
		"-S", fmt.Sprintf("-%d", scrollbackLines), // scrollback depth
	).Output()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane %s: %w", paneID, err)
	}
	return string(out), nil
}

// SendLiteral sends text as literal characters to a pane, without interpreting
// the text as tmux key names.
func SendLiteral(paneID, text string) error {
	if err := exec.Command("tmux", "send-keys", "-t", paneID, "-l", text).Run(); err != nil {
		return fmt.Errorf("tmux send-keys -l: %w", err)
	}
	return nil
}

// SendKeyName sends a named tmux key to a pane (e.g. "Enter", "C-c", "BSpace").
func SendKeyName(paneID, key string) error {
	if err := exec.Command("tmux", "send-keys", "-t", paneID, key).Run(); err != nil {
		return fmt.Errorf("tmux send-keys %s: %w", key, err)
	}
	return nil
}

// SendKeys sends text literally followed by Enter to a pane.
func SendKeys(paneID, text string) error {
	if err := SendLiteral(paneID, text); err != nil {
		return err
	}
	return SendKeyName(paneID, "Enter")
}

// ResizePane sets an explicit width on the window containing the pane.
// For single-pane windows (the common case for Claude sessions) resize-pane
// cannot shrink the pane below the window width, so we resize the window itself.
func ResizePane(paneID string, width int) error {
	if err := exec.Command("tmux", "resize-window", "-t", paneID, "-x", strconv.Itoa(width)).Run(); err != nil {
		return fmt.Errorf("tmux resize-window: %w", err)
	}
	return nil
}

// ResizeWindow sets explicit width and height on the window containing the pane.
func ResizeWindow(paneID string, width, height int) error {
	if err := exec.Command("tmux", "resize-window", "-t", paneID, "-x", strconv.Itoa(width), "-y", strconv.Itoa(height)).Run(); err != nil {
		return fmt.Errorf("tmux resize-window: %w", err)
	}
	return nil
}

// ResizePaneAuto removes any explicit size override on the window containing
// the pane, letting tmux fit it to the attached client naturally.
func ResizePaneAuto(paneID string) error {
	if err := exec.Command("tmux", "resize-window", "-A", "-t", paneID).Run(); err != nil {
		return fmt.Errorf("tmux resize-window -A: %w", err)
	}
	return nil
}

// SwitchToPane focuses the tmux client on the given pane, restoring its natural
// size first so it fills the terminal properly.
func SwitchToPane(paneID string) error {
	// Get current client dimensions and resize the target window to match.
	// This ensures the window fills the terminal properly even if it was
	// previously resized to a smaller size.
	clientWidth, wErr := ClientWidth()
	clientHeight, hErr := ClientHeight()
	if wErr == nil && hErr == nil && clientWidth > 0 && clientHeight > 0 {
		_ = ResizeWindow(paneID, clientWidth, clientHeight)
	}

	// select-window makes the window containing the pane active in its session.
	if err := exec.Command("tmux", "select-window", "-t", paneID).Run(); err != nil {
		return fmt.Errorf("tmux select-window: %w", err)
	}
	// select-pane makes this specific pane the active pane in that window.
	if err := exec.Command("tmux", "select-pane", "-t", paneID).Run(); err != nil {
		return fmt.Errorf("tmux select-pane: %w", err)
	}
	out, err := exec.Command("tmux", "display-message", "-t", paneID, "-p", "#{session_name}").Output()
	if err != nil {
		return fmt.Errorf("tmux display-message: %w", err)
	}
	sess := strings.TrimSpace(string(out))
	if err := exec.Command("tmux", "switch-client", "-t", sess).Run(); err != nil {
		return fmt.Errorf("tmux switch-client: %w", err)
	}
	return nil
}

// KillPane closes the given pane (and its window if it is the only pane).
func KillPane(paneID string) error {
	if err := exec.Command("tmux", "kill-pane", "-t", paneID).Run(); err != nil {
		return fmt.Errorf("tmux kill-pane: %w", err)
	}
	return nil
}

// NewWindow creates a new tmux window running cmd in path and returns the new pane ID.
func NewWindow(tmuxSession, path, cmd string) (string, error) {
	out, err := exec.Command(
		"tmux", "new-window",
		"-t", tmuxSession,
		"-c", path,
		"-P", "-F", "#{pane_id}",
		cmd,
	).Output()
	if err != nil {
		return "", fmt.Errorf("tmux new-window: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentSession returns the tmux session name herd is running in.
func CurrentSession() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return "", fmt.Errorf("not inside tmux: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// PaneWidth returns the current width of a pane.
func PaneWidth(paneID string) (int, error) {
	out, err := exec.Command("tmux", "display-message", "-t", paneID, "-p", "#{pane_width}").Output()
	if err != nil {
		return 0, err
	}
	w, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return w, nil
}

// ClientWidth returns the width of the current tmux client.
func ClientWidth() (int, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{client_width}").Output()
	if err != nil {
		return 0, err
	}
	w, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return w, nil
}

// ClientHeight returns the height of the current tmux client.
func ClientHeight() (int, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{client_height}").Output()
	if err != nil {
		return 0, err
	}
	h, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return h, nil
}
