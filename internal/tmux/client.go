package tmux

import (
	"fmt"
	"os"
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

// parsePaneLine parses a single tab-separated line from tmux list-panes output.
// Returns the Pane and true on success, or zero Pane and false if the line is malformed.
func parsePaneLine(line string) (Pane, bool) {
	f := strings.Split(line, "\t")
	if len(f) < 10 {
		return Pane{}, false
	}
	pid, _ := strconv.Atoi(f[5])
	wIdx, _ := strconv.Atoi(f[3])
	pIdx, _ := strconv.Atoi(f[4])
	w, _ := strconv.Atoi(f[8])
	h, _ := strconv.Atoi(f[9])
	return Pane{
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
	}, true
}

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
		if p, ok := parsePaneLine(line); ok {
			panes = append(panes, p)
		}
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
		"-p",                                      // print to stdout
		"-e",                                      // preserve SGR escape codes
		"-t", paneID,
		"-S", fmt.Sprintf("-%d", scrollbackLines), // scrollback depth
	).Output()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane %s: %w", paneID, err)
	}
	return string(out), nil
}

// CursorPosition returns the cursor X and Y position in a pane.
// X is the column (0-indexed), Y is the row (0-indexed from top of visible area).
func CursorPosition(paneID string) (x, y int, err error) {
	out, err := exec.Command(
		"tmux", "display", "-t", paneID, "-p", "#{cursor_x} #{cursor_y}",
	).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("tmux display cursor: %w", err)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected cursor output: %s", out)
	}
	x, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	y, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
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

// NewWindow creates a new tmux window in path, types cmd into the shell, and
// returns the new pane ID. The window is created detached (-d) so the client
// stays on the current window.
//
// We intentionally do NOT pass cmd as the window command. Doing so runs it
// directly without a shell, which means:
//   - The pane closes as soon as cmd exits (no shell survives to keep it open).
//   - On macOS, PATH may not include the user's shell profile entries (e.g.
//     Homebrew), so the binary may not be found or process naming may differ.
//
// Instead, we start the window with the user's default shell (no command), then
// send cmd as keystrokes. The shell remains after cmd exits and its full
// environment is available from the start.
func NewWindow(tmuxSession, path, cmd string) (string, error) {
	tmuxCmd := exec.Command(
		"tmux", "new-window",
		"-d", // detached — don't switch to the new window
		"-t", tmuxSession+":", // trailing colon = "this session, next window" (avoids numeric ambiguity)
		"-c", path,
		"-P", "-F", "#{pane_id}",
		// no command → tmux starts the user's default shell
	)
	out, err := tmuxCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("tmux new-window: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("tmux new-window: %w", err)
	}
	paneID := strings.TrimSpace(string(out))

	// Type the command into the shell. SendKeys sends it literally then presses
	// Enter, so the shell executes it while remaining alive afterwards.
	if err := SendKeys(paneID, cmd); err != nil {
		return paneID, fmt.Errorf("send command to new pane: %w", err)
	}
	return paneID, nil
}

// CurrentSession returns the tmux session name herd is running in.
// It targets $TMUX_PANE explicitly so the result is correct regardless of
// which client tmux considers "current".
func CurrentSession() (string, error) {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return "", fmt.Errorf("TMUX_PANE not set — is herd running inside tmux?")
	}
	out, err := exec.Command("tmux", "display-message", "-t", pane, "-p", "#{session_name}").Output()
	if err != nil {
		return "", fmt.Errorf("tmux display-message: %w", err)
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

// PaneHeight returns the current height of a pane.
func PaneHeight(paneID string) (int, error) {
	out, err := exec.Command("tmux", "display-message", "-t", paneID, "-p", "#{pane_height}").Output()
	if err != nil {
		return 0, err
	}
	h, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return h, nil
}

// PaneInfo returns the cursor position and pane height in a single tmux call,
// reducing the window for tearing between separate display-message invocations.
// cursorX is the column (0-indexed), cursorY is the row (0-indexed from top of
// visible area), paneHeight is the height of the pane in rows.
func PaneInfo(paneID string) (cursorX, cursorY, paneHeight int, err error) {
	out, err := exec.Command(
		"tmux", "display-message", "-t", paneID, "-p",
		"#{cursor_x} #{cursor_y} #{pane_height}",
	).Output()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("tmux display-message pane info: %w", err)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("unexpected pane info output: %s", out)
	}
	cursorX, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, err
	}
	cursorY, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, err
	}
	paneHeight, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, err
	}
	return cursorX, cursorY, paneHeight, nil
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
