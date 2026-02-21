package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shnupta/herd/internal/hook"
	"github.com/shnupta/herd/internal/state"
	"github.com/shnupta/herd/internal/tui"
)

// version is set by goreleaser via ldflags
var version = "dev"

const usage = `herd — tmux-based Claude Code session manager

Usage:
  herd                  Launch the TUI (must be run inside tmux)
  herd install          Install Claude Code hooks into ~/.claude/settings.json
  herd hook <event>     Handle a hook event (called by Claude Code, not directly)
  herd --help           Show this help

TUI key bindings:
  j / k / ↑ / ↓        Navigate sessions
  i                     Enter insert mode (forward keystrokes to Claude)
  ctrl+h                Exit insert mode
  t                     Jump to the selected pane in tmux
  r                     Refresh session list
  I                     Install hooks (same as 'herd install')
  q / ctrl+c            Quit

Status indicators:
  ●  working            Claude is using a tool
  ⏸  waiting            Claude is waiting for your input
  📋 plan ready         Claude has a plan awaiting approval
  🔔 notifying          Claude sent a notification
  ○  idle               Claude is idle

Status indicators require hooks to be installed ('herd install') and a fresh
Claude session started after installation.
`

func main() {
	// Subcommand: herd hook <EventType>
	// Called by Claude Code hooks — must be fast and produce no terminal output.
	if len(os.Args) >= 3 && os.Args[1] == "hook" {
		if err := hook.Run(os.Args[2]); err != nil {
			// Hooks must not fail loudly (Claude would surface the error).
			os.Exit(1)
		}
		return
	}

	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h" || os.Args[1] == "help") {
		fmt.Print(usage)
		return
	}

	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}

	// Subcommand: herd install
	// Writes herd hooks into ~/.claude/settings.json.
	if len(os.Args) == 2 && os.Args[1] == "install" {
		self, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error finding executable path:", err)
			os.Exit(1)
		}
		// Resolve any symlinks to get the real path
		self, err = filepath.EvalSymlinks(self)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error resolving executable path:", err)
			os.Exit(1)
		}
		if err := hook.Install(self); err != nil {
			fmt.Fprintln(os.Stderr, "error installing hooks:", err)
			os.Exit(1)
		}
		fmt.Printf("hooks installed → ~/.claude/settings.json\n")
		fmt.Printf("using herd at: %s\n", self)
		return
	}

	// Ensure we are running inside tmux.
	if os.Getenv("TMUX") == "" {
		fmt.Fprintln(os.Stderr, "herd must be run inside a tmux session")
		os.Exit(1)
	}

	// Start the state file watcher (best-effort; herd works without hooks).
	watcher, err := state.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not watch state dir: %v\n", err)
	}
	if watcher != nil {
		defer watcher.Close()
	}

	model := tui.New(watcher)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
