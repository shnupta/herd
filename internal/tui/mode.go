package tui

// Mode represents the current input mode of the TUI.
type Mode int

const (
	ModeNormal   Mode = iota
	ModeReview
	ModePicker
	ModeFilter
	ModeRename
	ModeGroupSet
)
