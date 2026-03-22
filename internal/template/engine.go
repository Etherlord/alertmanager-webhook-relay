package template

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const templateGlob = "*.html.tmpl"

// Engine loads and renders HTML templates with thread-safe access.
type Engine struct {
	dir    string
	mu     sync.RWMutex
	tmpl   *template.Template
	logger *slog.Logger
	funcs  template.FuncMap
}

// NewEngine creates a new template Engine that loads templates from dir.
// Returns an error if the directory does not exist or templates fail to parse.
func NewEngine(dir string, funcs template.FuncMap, logger *slog.Logger) (*Engine, error) {
	logger.Debug("template engine: initializing", "dir", dir)

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("template dir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("template dir %s: not a directory", dir)
	}

	e := &Engine{
		dir:    dir,
		logger: logger,
		funcs:  funcs,
	}

	if err := e.load(); err != nil {
		return nil, err
	}

	logger.Info("template engine: initialized", "dir", dir)
	return e, nil
}

// Render renders the named template with the given data.
// Thread-safe: multiple goroutines can call Render concurrently.
func (e *Engine) Render(name string, data any) (string, error) {
	e.mu.RLock()
	tmpl := e.tmpl
	e.mu.RUnlock()

	if tmpl == nil {
		return "", fmt.Errorf("template engine: no templates loaded")
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("template render %s: %w", name, err)
	}

	e.logger.Debug("template engine: rendered", "name", name, "size", buf.Len())
	return buf.String(), nil
}

// Reload reloads all templates from the directory.
// Fail-safe: if loading fails, the previous templates remain active.
func (e *Engine) Reload() error {
	e.logger.Debug("template engine: reloading", "dir", e.dir)

	if err := e.load(); err != nil {
		e.logger.Error("template engine: reload failed, keeping previous templates",
			"dir", e.dir,
			"error", err,
		)
		return err
	}

	e.logger.Info("template engine: reloaded", "dir", e.dir)
	return nil
}

// load parses all templates from the directory and atomically swaps the cache.
func (e *Engine) load() error {
	pattern := filepath.Join(e.dir, templateGlob)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("template glob %s: %w", pattern, err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("template dir %s: no %s files found", e.dir, templateGlob)
	}

	tmpl, err := template.New("").Funcs(e.funcs).ParseGlob(pattern)
	if err != nil {
		return fmt.Errorf("template parse %s: %w", pattern, err)
	}

	e.mu.Lock()
	e.tmpl = tmpl
	e.mu.Unlock()

	e.logger.Debug("template engine: loaded templates",
		"dir", e.dir,
		"count", len(matches),
	)
	return nil
}
