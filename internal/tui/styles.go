package tui

import "github.com/charmbracelet/lipgloss"

const sessionPaneWidth = 28

var (
	// Colours
	colAccent  = lipgloss.Color("#7C3AED") // purple
	colWorking = lipgloss.Color("#F59E0B") // amber
	colWaiting = lipgloss.Color("#3B82F6") // blue
	colIdle    = lipgloss.Color("#6B7280") // grey
	colPlan    = lipgloss.Color("#10B981") // emerald
	colText    = lipgloss.Color("#E5E7EB")
	colSubtext = lipgloss.Color("#6B7280")
	colBorder  = lipgloss.Color("#374151")
	colSelected = lipgloss.Color("#1F2937")

	styleHeader = lipgloss.NewStyle().
			Background(colAccent).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	styleSessionPane = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderRight(true).
				BorderForeground(colBorder)

	styleSessionItem = lipgloss.NewStyle().
				Foreground(colText).
				PaddingLeft(1)

	styleSessionItemSelected = lipgloss.NewStyle().
					Background(colSelected).
					Foreground(colText).
					Bold(true).
					PaddingLeft(1)

	styleSessionMeta = lipgloss.NewStyle().
				Foreground(colSubtext).
				PaddingLeft(3)

	styleOutputHeader = lipgloss.NewStyle().
				Foreground(colSubtext).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(colBorder)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(colSubtext).
			PaddingLeft(1)

	styleHelp = lipgloss.NewStyle().
			Background(colSelected).
			Foreground(colSubtext).
			PaddingLeft(1)

	styleHelpInsert = lipgloss.NewStyle().
			Background(lipgloss.Color("#3D2E0A")).
			Foreground(lipgloss.Color("#FDE68A")).
			PaddingLeft(1)
)

// stateIcon returns a coloured indicator for the session state.
func stateIcon(stateStr string) string {
	switch stateStr {
	case "working":
		return lipgloss.NewStyle().Foreground(colWorking).Render("●")
	case "waiting":
		return lipgloss.NewStyle().Foreground(colWaiting).Render("⏸")
	case "plan_ready":
		return lipgloss.NewStyle().Foreground(colPlan).Render("📋")
	case "notifying":
		return lipgloss.NewStyle().Foreground(colWaiting).Render("🔔")
	case "idle":
		return lipgloss.NewStyle().Foreground(colIdle).Render("○")
	default:
		return lipgloss.NewStyle().Foreground(colSubtext).Render("?")
	}
}

func stateLabel(stateStr, tool string) string {
	switch stateStr {
	case "working":
		if tool != "" {
			return lipgloss.NewStyle().Foreground(colWorking).Render(tool)
		}
		return lipgloss.NewStyle().Foreground(colWorking).Render("working")
	case "waiting":
		return lipgloss.NewStyle().Foreground(colWaiting).Render("waiting")
	case "plan_ready":
		return lipgloss.NewStyle().Foreground(colPlan).Render("plan ready")
	case "idle":
		return lipgloss.NewStyle().Foreground(colIdle).Render("idle")
	default:
		return lipgloss.NewStyle().Foreground(colSubtext).Render("—")
	}
}
