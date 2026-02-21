package state

import (
	"os"

	"github.com/fsnotify/fsnotify"
)

// WatcherIface is implemented by Watcher and any test double.
type WatcherIface interface {
	// Events returns the channel on which state updates are delivered.
	Events() <-chan SessionState
	// Close stops the watcher and releases resources.
	Close()
}

// compile-time check
var _ WatcherIface = (*Watcher)(nil)

// Watcher watches ~/.herd/sessions/ for state file changes.
type Watcher struct {
	events chan SessionState
	errors chan error
	done   chan struct{}
	fw     *fsnotify.Watcher
	store  *Store
}

// Events returns the channel on which state updates are delivered.
func (w *Watcher) Events() <-chan SessionState { return w.events }

// NewWatcher creates and starts a file watcher on the default state directory.
func NewWatcher() (*Watcher, error) {
	return NewWatcherForStore(defaultStore)
}

// NewWatcherForStore creates and starts a file watcher on the given store's directory.
func NewWatcherForStore(store *Store) (*Watcher, error) {
	if err := os.MkdirAll(store.Dir(), 0o755); err != nil {
		return nil, err
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fw.Add(store.Dir()); err != nil {
		fw.Close()
		return nil, err
	}

	w := &Watcher{
		events: make(chan SessionState, 16),
		errors: make(chan error, 4),
		done:   make(chan struct{}),
		fw:     fw,
		store:  store,
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) loop() {
	defer close(w.events)
	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.fw.Events:
			if !ok {
				return
			}
			// Only care about writes/creates of .json files (not .tmp)
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			if len(event.Name) < 5 || event.Name[len(event.Name)-5:] != ".json" {
				continue
			}
			states, err := w.store.ReadAll()
			if err != nil {
				continue
			}
			for _, s := range states {
				if w.store.Path(s.SessionID) == event.Name {
					select {
					case w.events <- s:
					default:
					}
					break
				}
			}
		case err, ok := <-w.fw.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			default:
			}
		}
	}
}

// Close stops the watcher.
func (w *Watcher) Close() {
	close(w.done)
	w.fw.Close()
}
