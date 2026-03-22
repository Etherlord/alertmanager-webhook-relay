package template

import (
	"context"
	"log/slog"
	"time"

	"github.com/fsnotify/fsnotify"
)

const defaultDebounce = 500 * time.Millisecond

// Watcher watches the template directory for changes and reloads the engine.
type Watcher struct {
	engine   *Engine
	debounce time.Duration
	logger   *slog.Logger
}

// NewWatcher creates a new template Watcher.
func NewWatcher(engine *Engine, logger *slog.Logger) *Watcher {
	return &Watcher{
		engine:   engine,
		debounce: defaultDebounce,
		logger:   logger,
	}
}

// Watch starts watching the template directory for changes.
// Blocks until ctx is cancelled. Returns nil on graceful shutdown.
func (w *Watcher) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(w.engine.dir); err != nil {
		return err
	}

	w.logger.Info("template watcher: started", "dir", w.engine.dir, "debounce", w.debounce)

	var timer *time.Timer
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("template watcher: stopping")
			if timer != nil {
				timer.Stop()
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}

			w.logger.Debug("template watcher: file changed",
				"file", event.Name,
				"op", event.Op.String(),
			)

			// Debounce: reset timer on each event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, func() {
				w.logger.Debug("template watcher: reloading after debounce")
				if err := w.engine.Reload(); err != nil {
					w.logger.Error("template watcher: reload failed", "error", err)
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("template watcher: error", "error", err)
		}
	}
}
