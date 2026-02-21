package tui

import "github.com/shnupta/herd/internal/tmux"

// mockTmuxClient is a test double for tmux.ClientIface.
type mockTmuxClient struct {
	panes             []tmux.Pane
	listPanesErr      error
	captureOutput     string
	captureErr        error
	cursorX, cursorY  int
	paneWidthVal      int
	paneHeightVal     int
	clientWidthVal    int
	clientHeightVal   int
	currentSessionVal string
}

var _ tmux.ClientIface = (*mockTmuxClient)(nil)

func (m *mockTmuxClient) ListPanes() ([]tmux.Pane, error) { return m.panes, m.listPanesErr }
func (m *mockTmuxClient) CapturePane(string, int) (string, error) {
	return m.captureOutput, m.captureErr
}
func (m *mockTmuxClient) CursorPosition(string) (int, int, error) {
	return m.cursorX, m.cursorY, nil
}
func (m *mockTmuxClient) SendLiteral(string, string) error      { return nil }
func (m *mockTmuxClient) SendKeyName(string, string) error       { return nil }
func (m *mockTmuxClient) SendKeys(string, string) error          { return nil }
func (m *mockTmuxClient) ResizePane(string, int) error           { return nil }
func (m *mockTmuxClient) ResizeWindow(string, int, int) error    { return nil }
func (m *mockTmuxClient) ResizePaneAuto(string) error            { return nil }
func (m *mockTmuxClient) SwitchToPane(string) error              { return nil }
func (m *mockTmuxClient) KillPane(string) error                  { return nil }
func (m *mockTmuxClient) NewWindow(string, string, string) (string, error) {
	return "", nil
}
func (m *mockTmuxClient) CurrentSession() (string, error) {
	return m.currentSessionVal, nil
}
func (m *mockTmuxClient) PaneWidth(string) (int, error)  { return m.paneWidthVal, nil }
func (m *mockTmuxClient) PaneHeight(string) (int, error) { return m.paneHeightVal, nil }
func (m *mockTmuxClient) PaneInfo(string) (int, int, int, error) {
	return m.cursorX, m.cursorY, m.paneHeightVal, nil
}
func (m *mockTmuxClient) ClientWidth() (int, error)  { return m.clientWidthVal, nil }
func (m *mockTmuxClient) ClientHeight() (int, error) { return m.clientHeightVal, nil }
