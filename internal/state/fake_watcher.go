package state

// FakeWatcher is a WatcherIface implementation for tests.
// Push state updates via Send() to simulate file system events.
type FakeWatcher struct {
	ch chan SessionState
}

// compile-time check
var _ WatcherIface = (*FakeWatcher)(nil)

// NewFakeWatcher creates a FakeWatcher with a buffered channel.
func NewFakeWatcher() *FakeWatcher {
	return &FakeWatcher{ch: make(chan SessionState, 16)}
}

// Events returns the channel on which state updates are delivered.
func (f *FakeWatcher) Events() <-chan SessionState { return f.ch }

// Close closes the events channel.
func (f *FakeWatcher) Close() { close(f.ch) }

// Send pushes a state update into the events channel.
func (f *FakeWatcher) Send(s SessionState) { f.ch <- s }
