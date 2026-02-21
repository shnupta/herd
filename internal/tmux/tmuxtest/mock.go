// Package tmuxtest provides test doubles for the tmux package.
// Import this package only from _test.go files.
package tmuxtest

import "github.com/shnupta/herd/internal/tmux"

// MockClient is a test double for tmux.ClientIface.
// Set fields before calling methods to control return values.
type MockClient struct {
	Panes        []tmux.Pane
	ListPanesErr error

	CaptureOutput string
	CaptureErr    error

	CursorX   int
	CursorY   int
	CursorErr error

	PaneWidthVal  int
	PaneWidthErr  error
	PaneHeightVal int
	PaneHeightErr error

	PaneInfoCursorX int
	PaneInfoCursorY int
	PaneInfoHeight  int
	PaneInfoErr     error

	ClientWidthVal  int
	ClientWidthErr  error
	ClientHeightVal int
	ClientHeightErr error

	CurrentSessionVal string
	CurrentSessionErr error

	NewWindowPane string
	NewWindowErr  error

	ResizePaneErr     error
	ResizeWindowErr   error
	ResizePaneAutoErr error
	SwitchToPaneErr   error
	KillPaneErr       error
	SendLiteralErr    error
	SendKeyNameErr    error
	SendKeysErr       error

	// Track calls for assertions.
	SendLiteralCalls []string
	SendKeyCalls     []string
	SendKeysCalls    []string
	KilledPanes      []string
	SwitchedPanes    []string
}

// Compile-time check that MockClient satisfies tmux.ClientIface.
var _ tmux.ClientIface = (*MockClient)(nil)

func (m *MockClient) ListPanes() ([]tmux.Pane, error) {
	return m.Panes, m.ListPanesErr
}

func (m *MockClient) CapturePane(paneID string, scrollbackLines int) (string, error) {
	return m.CaptureOutput, m.CaptureErr
}

func (m *MockClient) CursorPosition(paneID string) (x, y int, err error) {
	return m.CursorX, m.CursorY, m.CursorErr
}

func (m *MockClient) SendLiteral(paneID, text string) error {
	m.SendLiteralCalls = append(m.SendLiteralCalls, paneID+":"+text)
	return m.SendLiteralErr
}

func (m *MockClient) SendKeyName(paneID, key string) error {
	m.SendKeyCalls = append(m.SendKeyCalls, paneID+":"+key)
	return m.SendKeyNameErr
}

func (m *MockClient) SendKeys(paneID, text string) error {
	m.SendKeysCalls = append(m.SendKeysCalls, paneID+":"+text)
	return m.SendKeysErr
}

func (m *MockClient) ResizePane(paneID string, width int) error {
	return m.ResizePaneErr
}

func (m *MockClient) ResizeWindow(paneID string, width, height int) error {
	return m.ResizeWindowErr
}

func (m *MockClient) ResizePaneAuto(paneID string) error {
	return m.ResizePaneAutoErr
}

func (m *MockClient) SwitchToPane(paneID string) error {
	m.SwitchedPanes = append(m.SwitchedPanes, paneID)
	return m.SwitchToPaneErr
}

func (m *MockClient) KillPane(paneID string) error {
	m.KilledPanes = append(m.KilledPanes, paneID)
	return m.KillPaneErr
}

func (m *MockClient) NewWindow(tmuxSession, path, cmd string) (string, error) {
	return m.NewWindowPane, m.NewWindowErr
}

func (m *MockClient) CurrentSession() (string, error) {
	return m.CurrentSessionVal, m.CurrentSessionErr
}

func (m *MockClient) PaneWidth(paneID string) (int, error) {
	return m.PaneWidthVal, m.PaneWidthErr
}

func (m *MockClient) PaneHeight(paneID string) (int, error) {
	return m.PaneHeightVal, m.PaneHeightErr
}

func (m *MockClient) PaneInfo(paneID string) (cursorX, cursorY, paneHeight int, err error) {
	return m.PaneInfoCursorX, m.PaneInfoCursorY, m.PaneInfoHeight, m.PaneInfoErr
}

func (m *MockClient) ClientWidth() (int, error) {
	return m.ClientWidthVal, m.ClientWidthErr
}

func (m *MockClient) ClientHeight() (int, error) {
	return m.ClientHeightVal, m.ClientHeightErr
}
