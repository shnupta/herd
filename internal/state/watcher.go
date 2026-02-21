package state

import (
	"os"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches ~/.herd/sessions/ for state file changes.
type Watcher struct {
	Events chan SessionState
	Errors chan error
	done   chan struct{}
	fw     *fsnotify.Watcher
	store  *Store
}

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
		Events: make(chan SessionState, 16),
		Errors: make(chan error, 4),
		done:   make(chan struct{}),
		fw:     fw,
		store:  store,
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) loop() {
	defer close(w.Events)
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
					case w.Events <- s:
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
			case w.Errors <- err:
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
