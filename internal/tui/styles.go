package tui

import "github.com/charmbracelet/lipgloss"

const sessionPaneWidth = 28

var (
	// ── Palette ──────────────────────────────────────────────────────────────
	colBg        = lipgloss.Color("#0D1117")
	colSurface   = lipgloss.Color("#161B22")
	colBorder    = lipgloss.Color("#30363D")

	colText      = lipgloss.Color("#E6EDF3")
	colSubtext   = lipgloss.Color("#8B949E")
	colSubtle    = lipgloss.Color("#484F58")

	colAccent    = lipgloss.Color("#7C3AED") // purple header
	colGold      = lipgloss.Color("#F0B429") // selected
	colGoldDim   = lipgloss.Color("#2D2200") // selected bg
	colGoldText  = lipgloss.Color("#FDE68A") // selected text

	colGreen     = lipgloss.Color("#3FB950") // working / added
	colGreenDim  = lipgloss.Color("#1A4027")
	colBlue      = lipgloss.Color("#58A6FF") // waiting / file headers
	colBlueDim   = lipgloss.Color("#0D2044")
	colAmber     = lipgloss.Color("#FFA657") // plan ready / comments
	colAmberDim  = lipgloss.Color("#2D1A00")
	colRed       = lipgloss.Color("#F85149") // removed lines
	colRedDim    = lipgloss.Color("#3D0A08")
	colPurple    = lipgloss.Color("#BC8CFF") // notifying
	colCyan      = lipgloss.Color("#39C5CF") // idle
	colMuted     = lipgloss.Color("#6B7280")

	colGroupedBg = lipgloss.Color("#0D1117")
	colSelected  = colGoldDim

	// ── Main header ──────────────────────────────────────────────────────────
	styleHeader = lipgloss.NewStyle().
			Background(colAccent).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	// ── Session pane ─────────────────────────────────────────────────────────
	styleSessionPane = lipgloss.NewStyle().
				BorderStyle(lipgloss.ThickBorder()).
				BorderRight(true).
				BorderForeground(colBorder)

	styleSessionItem = lipgloss.NewStyle().
				Foreground(colText).
				PaddingLeft(1)

	styleSessionItemSelected = lipgloss.NewStyle().
					Background(colGoldDim).
					Foreground(colGoldText).
					Bold(true).
					PaddingLeft(1)

	styleSessionMeta = lipgloss.NewStyle().
				Foreground(colSubtext).
				PaddingLeft(3)

	// ── Output pane ──────────────────────────────────────────────────────────
	styleOutputHeader = lipgloss.NewStyle().
				Foreground(colSubtext).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(colBorder)

	// ── Status / help bars ───────────────────────────────────────────────────
	styleStatusBar = lipgloss.NewStyle().
			Foreground(colSubtext).
			PaddingLeft(1)

	styleHelp = lipgloss.NewStyle().
			Background(colSurface).
			Foreground(colSubtext).
			PaddingLeft(1)

	styleHelpInsert = lipgloss.NewStyle().
			Background(colBlueDim).
			Foreground(colBlue).
			Bold(true).
			PaddingLeft(1)

	styleHelpFilter = lipgloss.NewStyle().
			Background(colAmberDim).
			Foreground(colAmber).
			Bold(true).
			PaddingLeft(1)

	// ── Group headers ────────────────────────────────────────────────────────
	styleGroupHeader = lipgloss.NewStyle().
				Foreground(colSubtext).
				PaddingLeft(1)

	styleGroupHeaderSelected = lipgloss.NewStyle().
					Background(colGoldDim).
					Foreground(colGoldText).
					Bold(true).
					PaddingLeft(1)

	// ── Overlay inputs ───────────────────────────────────────────────────────
	styleOverlayTitle = lipgloss.NewStyle().
				Background(colAccent).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true).
				Padding(0, 1)

	styleOverlayInput = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colGold).
				Padding(0, 1)

	styleOverlayHelp = lipgloss.NewStyle().
				Foreground(colSubtext).
				PaddingLeft(1)

	// ── Filter input ─────────────────────────────────────────────────────────
	styleFilter = lipgloss.NewStyle().
			Foreground(colAmber).
			Bold(true).
			PaddingLeft(1)
)

// stateIcon returns a coloured indicator for the session state.
func stateIcon(stateStr string) string {
	switch stateStr {
	case "working":
		return lipgloss.NewStyle().Foreground(colGreen).Render("●")
	case "waiting":
		return lipgloss.NewStyle().Foreground(colBlue).Render("◉")
	case "plan_ready":
		return lipgloss.NewStyle().Foreground(colAmber).Render("◆")
	case "notifying":
		return lipgloss.NewStyle().Foreground(colPurple).Render("◈")
	case "idle":
		return lipgloss.NewStyle().Foreground(colCyan).Render("○")
	default:
		return lipgloss.NewStyle().Foreground(colSubtle).Render("·")
	}
}

func stateLabel(stateStr, tool string) string {
	switch stateStr {
	case "working":
		s := lipgloss.NewStyle().Foreground(colGreen).Bold(true)
		if tool != "" {
			return s.Render(tool)
		}
		return s.Render("working")
	case "waiting":
		return lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("waiting")
	case "plan_ready":
		return lipgloss.NewStyle().Foreground(colAmber).Bold(true).Render("plan ready")
	case "notifying":
		return lipgloss.NewStyle().Foreground(colPurple).Bold(true).Render("notifying")
	case "idle":
		return lipgloss.NewStyle().Foreground(colCyan).Render("idle")
	default:
		return lipgloss.NewStyle().Foreground(colSubtle).Render("—")
	}
}

// stateBg returns a subtle background tint for a session row based on state.
func stateBg(stateStr string) lipgloss.Color {
	switch stateStr {
	case "working":
		return colGreenDim
	case "waiting":
		return colBlueDim
	case "plan_ready":
		return colAmberDim
	case "notifying":
		return lipgloss.Color("#1A0D2E")
	default:
		return colBg
	}
}
