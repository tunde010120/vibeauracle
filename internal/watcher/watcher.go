// Package watcher provides a high-speed, decoupled filesystem event system.
// It is designed to be lightning fast and integrates with the daemon for
// real-time event distribution to any part of the application (tree view, caches, etc).
package watcher

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventType represents the kind of filesystem event.
type EventType int

const (
	EventCreate EventType = iota
	EventWrite
	EventRemove
	EventRename
	EventChmod
)

func (e EventType) String() string {
	switch e {
	case EventCreate:
		return "CREATE"
	case EventWrite:
		return "WRITE"
	case EventRemove:
		return "REMOVE"
	case EventRename:
		return "RENAME"
	case EventChmod:
		return "CHMOD"
	default:
		return "UNKNOWN"
	}
}

// Event represents a single filesystem change.
type Event struct {
	Type      EventType
	Path      string
	Timestamp time.Time
}

// Subscriber is any component that wants to receive filesystem events.
type Subscriber interface {
	OnFileEvent(event Event)
}

// SubscriberFunc is a function adapter for Subscriber.
type SubscriberFunc func(Event)

func (f SubscriberFunc) OnFileEvent(event Event) { f(event) }

// Watcher is a high-speed filesystem event hub.
// It watches directories recursively and broadcasts events to all subscribers.
type Watcher struct {
	mu             sync.RWMutex
	watcher        *fsnotify.Watcher
	subscribers    []Subscriber
	roots          map[string]bool
	ignorePatterns []string
	debounceMap    map[string]time.Time
	debounceDur    time.Duration
	stopCh         chan struct{}
	running        bool
}

// New creates a new filesystem watcher.
func New() (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		watcher:        w,
		subscribers:    make([]Subscriber, 0),
		roots:          make(map[string]bool),
		ignorePatterns: defaultIgnorePatterns(),
		debounceMap:    make(map[string]time.Time),
		debounceDur:    50 * time.Millisecond, // 50ms debounce for rapid saves
		stopCh:         make(chan struct{}),
	}, nil
}

// defaultIgnorePatterns returns common patterns to ignore (build artifacts, etc).
func defaultIgnorePatterns() []string {
	return []string{
		".git",
		"node_modules",
		"__pycache__",
		".venv",
		"vendor",
		"*.swp",
		"*.swo",
		"*~",
		".DS_Store",
		"*.log",
	}
}

// Subscribe adds a new event listener.
func (w *Watcher) Subscribe(s Subscriber) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.subscribers = append(w.subscribers, s)
}

// SubscribeFunc adds a function as an event listener.
func (w *Watcher) SubscribeFunc(f func(Event)) {
	w.Subscribe(SubscriberFunc(f))
}

// AddRoot adds a directory to watch recursively.
func (w *Watcher) AddRoot(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.roots[absPath] = true
	w.mu.Unlock()

	return w.addRecursive(absPath)
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !d.IsDir() {
			return nil
		}

		// Skip ignored patterns
		base := filepath.Base(path)
		for _, pattern := range w.ignorePatterns {
			if matched, _ := filepath.Match(pattern, base); matched {
				return filepath.SkipDir
			}
			if strings.HasPrefix(base, ".") && pattern == ".git" {
				// Skip all hidden dirs for performance
				return filepath.SkipDir
			}
		}

		return w.watcher.Add(path)
	})
}

// RemoveRoot removes a directory from watch.
func (w *Watcher) RemoveRoot(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	w.mu.Lock()
	delete(w.roots, absPath)
	w.mu.Unlock()

	return w.watcher.Remove(absPath)
}

// Start begins the event loop. Non-blocking.
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.eventLoop()
}

// Stop halts the watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	close(w.stopCh)
	w.watcher.Close()
	w.running = false
}

func (w *Watcher) eventLoop() {
	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Log errors silently; don't crash the loop
		}
	}
}

func (w *Watcher) handleEvent(raw fsnotify.Event) {
	// Debounce rapid events on the same file
	w.mu.Lock()
	if lastTime, ok := w.debounceMap[raw.Name]; ok {
		if time.Since(lastTime) < w.debounceDur {
			w.mu.Unlock()
			return
		}
	}
	w.debounceMap[raw.Name] = time.Now()
	w.mu.Unlock()

	// Convert to our event type
	evt := Event{
		Path:      raw.Name,
		Timestamp: time.Now(),
	}

	switch {
	case raw.Op&fsnotify.Create != 0:
		evt.Type = EventCreate
		// If a directory was created, add it to the watcher
		w.addRecursive(raw.Name)
	case raw.Op&fsnotify.Write != 0:
		evt.Type = EventWrite
	case raw.Op&fsnotify.Remove != 0:
		evt.Type = EventRemove
	case raw.Op&fsnotify.Rename != 0:
		evt.Type = EventRename
	case raw.Op&fsnotify.Chmod != 0:
		evt.Type = EventChmod
	default:
		return // Unknown event, skip
	}

	// Broadcast to all subscribers (concurrent-safe read)
	w.mu.RLock()
	subs := make([]Subscriber, len(w.subscribers))
	copy(subs, w.subscribers)
	w.mu.RUnlock()

	for _, sub := range subs {
		go sub.OnFileEvent(evt) // Non-blocking broadcast
	}
}

// ForceReload triggers an artificial event for components that need a manual refresh.
func (w *Watcher) ForceReload(path string) {
	evt := Event{
		Type:      EventWrite,
		Path:      path,
		Timestamp: time.Now(),
	}

	w.mu.RLock()
	subs := make([]Subscriber, len(w.subscribers))
	copy(subs, w.subscribers)
	w.mu.RUnlock()

	for _, sub := range subs {
		go sub.OnFileEvent(evt)
	}
}

// ReloadAll sends a synthetic event for every root directory.
func (w *Watcher) ReloadAll() {
	w.mu.RLock()
	roots := make([]string, 0, len(w.roots))
	for r := range w.roots {
		roots = append(roots, r)
	}
	w.mu.RUnlock()

	for _, root := range roots {
		w.ForceReload(root)
	}
}
