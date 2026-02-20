package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Jump     key.Binding
	Insert   key.Binding
	New      key.Binding
	Kill     key.Binding
	Worktree key.Binding
	Refresh  key.Binding
	Quit     key.Binding
	Install  key.Binding
	Review   key.Binding
	Filter   key.Binding
	Pin      key.Binding
	MoveUp   key.Binding
	MoveDown key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "down"),
	),
	Jump: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "jump to pane"),
	),
	Insert: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "insert mode"),
	),
	New: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new session"),
	),
	Kill: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "kill session"),
	),
	Worktree: key.NewBinding(
		key.WithKeys("w"),
		key.WithHelp("w", "worktrees"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Install: key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("I", "install hooks"),
	),
	Review: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "diff review"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Pin: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "pin/unpin"),
	),
	MoveUp: key.NewBinding(
		key.WithKeys("K", "shift+up"),
		key.WithHelp("K", "move up"),
	),
	MoveDown: key.NewBinding(
		key.WithKeys("J", "shift+down"),
		key.WithHelp("J", "move down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}
