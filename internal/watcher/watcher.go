package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/fsnotify/fsnotify"
)

// CheckpointCallback is called when an automatic checkpoint is created
type CheckpointCallback func(cp *core.Checkpoint)

// Watcher monitors a workspace for file mutations and automatically creates checkpoints
type Watcher struct {
	ws           *core.Workspace
	fsw          *fsnotify.Watcher
	debounce     time.Duration
	agent        string
	onCheckpoint CheckpointCallback
	mu           sync.Mutex
	stopCh       chan struct{}
}

// NewWatcher initializes a file watcher with debounced auto-checkpointing
func NewWatcher(ws *core.Workspace, debounce time.Duration, agent string, cb CheckpointCallback) (*Watcher, error) {
	if debounce <= 0 {
		debounce = 1 * time.Second
	}
	if agent == "" {
		agent = "auto-watcher"
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	w := &Watcher{
		ws:           ws,
		fsw:          fsw,
		debounce:     debounce,
		agent:        agent,
		onCheckpoint: cb,
		stopCh:       make(chan struct{}),
	}

	if err := w.watchDirectories(); err != nil {
		_ = fsw.Close()
		return nil, err
	}

	return w, nil
}

func (w *Watcher) watchDirectories() error {
	return filepath.WalkDir(w.ws.RootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(w.ws.RootDir, path)
		if err != nil {
			return nil
		}

		cleanRel := filepath.ToSlash(relPath)
		if cleanRel != "." && w.ws.Matcher.Match(cleanRel, true) {
			return fs.SkipDir
		}

		return w.fsw.Add(path)
	})
}

// Start begins listening for file events and automatically checkpoints after file activity settles
func (w *Watcher) Start(ctx context.Context) error {
	var timer *time.Timer
	var timerMu sync.Mutex

	triggerCheckpoint := func() {
		w.mu.Lock()
		defer w.mu.Unlock()

		cp, err := core.AutoCheckpointIfDirty(w.ws, "file activity settled", w.agent)
		if err == nil && cp != nil {
			_ = index.IndexWorkspaceCheckpoint(w.ws, cp)
			if w.onCheckpoint != nil {
				w.onCheckpoint(cp)
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.stopCh:
			return nil
		case event, ok := <-w.fsw.Events:
			if !ok {
				return nil
			}

			relPath, err := filepath.Rel(w.ws.RootDir, event.Name)
			if err != nil {
				continue
			}
			cleanRel := filepath.ToSlash(relPath)

			// Ignore events on ignored paths (e.g. .echo/, .git/, build/, etc.)
			if w.ws.Matcher.Match(cleanRel, false) {
				continue
			}

			// If new directory created, watch it
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if !w.ws.Matcher.Match(cleanRel, true) {
						_ = w.fsw.Add(event.Name)
					}
					continue
				}
			}

			// Debounce file mutations
			timerMu.Lock()
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, triggerCheckpoint)
			timerMu.Unlock()

		case _, ok := <-w.fsw.Errors:
			if !ok {
				return nil
			}
		}
	}
}

// Stop shuts down the watcher
func (w *Watcher) Stop() error {
	close(w.stopCh)
	return w.fsw.Close()
}
