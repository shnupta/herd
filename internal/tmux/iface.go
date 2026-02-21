package tmux

// ClientIface defines the tmux operations used by herd.
// Enables mocking in tests without a real tmux server.
type ClientIface interface {
	ListPanes() ([]Pane, error)
	CapturePane(paneID string, scrollbackLines int) (string, error)
	CursorPosition(paneID string) (x, y int, err error)
	SendLiteral(paneID, text string) error
	SendKeyName(paneID, key string) error
	SendKeys(paneID, text string) error
	ResizePane(paneID string, width int) error
	ResizeWindow(paneID string, width, height int) error
	ResizePaneAuto(paneID string) error
	SwitchToPane(paneID string) error
	KillPane(paneID string) error
	NewWindow(tmuxSession, path, cmd string) (string, error)
	CurrentSession() (string, error)
	PaneWidth(paneID string) (int, error)
	PaneHeight(paneID string) (int, error)
	PaneInfo(paneID string) (cursorX, cursorY, paneHeight int, err error)
	ClientWidth() (int, error)
	ClientHeight() (int, error)
}

// Client implements ClientIface by shelling out to real tmux commands.
type Client struct{}

// Compile-time check that Client satisfies ClientIface.
var _ ClientIface = (*Client)(nil)

func (c *Client) ListPanes() ([]Pane, error)                                    { return ListPanes() }
func (c *Client) CapturePane(paneID string, scrollbackLines int) (string, error) { return CapturePane(paneID, scrollbackLines) }
func (c *Client) CursorPosition(paneID string) (int, int, error)                { return CursorPosition(paneID) }
func (c *Client) SendLiteral(paneID, text string) error                         { return SendLiteral(paneID, text) }
func (c *Client) SendKeyName(paneID, key string) error                          { return SendKeyName(paneID, key) }
func (c *Client) SendKeys(paneID, text string) error                            { return SendKeys(paneID, text) }
func (c *Client) ResizePane(paneID string, width int) error                     { return ResizePane(paneID, width) }
func (c *Client) ResizeWindow(paneID string, width, height int) error           { return ResizeWindow(paneID, width, height) }
func (c *Client) ResizePaneAuto(paneID string) error                            { return ResizePaneAuto(paneID) }
func (c *Client) SwitchToPane(paneID string) error                              { return SwitchToPane(paneID) }
func (c *Client) KillPane(paneID string) error                                  { return KillPane(paneID) }
func (c *Client) NewWindow(tmuxSession, path, cmd string) (string, error)       { return NewWindow(tmuxSession, path, cmd) }
func (c *Client) CurrentSession() (string, error)                               { return CurrentSession() }
func (c *Client) PaneWidth(paneID string) (int, error)                          { return PaneWidth(paneID) }
func (c *Client) PaneHeight(paneID string) (int, error)                         { return PaneHeight(paneID) }
func (c *Client) PaneInfo(paneID string) (int, int, int, error)                 { return PaneInfo(paneID) }
func (c *Client) ClientWidth() (int, error)                                     { return ClientWidth() }
func (c *Client) ClientHeight() (int, error)                                    { return ClientHeight() }
