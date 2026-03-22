package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_DetectsChange(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"watch.html.tmpl": `<p>Version 1</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	// Verify initial render.
	r1, err := engine.Render("watch.html.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>Version 1</p>", r1)

	w := NewWatcher(engine, testLogger())
	w.debounce = 100 * time.Millisecond // Shorter for tests.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx)
	}()

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Modify the template.
	err = os.WriteFile(filepath.Join(dir, "watch.html.tmpl"), []byte(`<p>Version 2</p>`), 0o644)
	require.NoError(t, err)

	// Wait for debounce + reload.
	time.Sleep(500 * time.Millisecond)

	// Verify updated render.
	r2, err := engine.Render("watch.html.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>Version 2</p>", r2)

	cancel()
	assert.NoError(t, <-done)
}

func TestWatcher_Debounce(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"debounce.html.tmpl": `<p>V0</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	w := NewWatcher(engine, testLogger())
	w.debounce = 200 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Rapid-fire multiple changes.
	for i := range 5 {
		_ = i
		err = os.WriteFile(filepath.Join(dir, "debounce.html.tmpl"), []byte(`<p>interim</p>`), 0o644)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)
	}

	// Final write.
	err = os.WriteFile(filepath.Join(dir, "debounce.html.tmpl"), []byte(`<p>Final</p>`), 0o644)
	require.NoError(t, err)

	// Wait for debounce.
	time.Sleep(500 * time.Millisecond)

	r, err := engine.Render("debounce.html.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>Final</p>", r)

	cancel()
	assert.NoError(t, <-done)
}

func TestWatcher_GracefulShutdown(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"shutdown.html.tmpl": `<p>OK</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	w := NewWatcher(engine, testLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop within timeout")
	}
}

func TestWatcher_ErrorResilience(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"resilient.html.tmpl": `<p>Original</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	w := NewWatcher(engine, testLogger())
	w.debounce = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Write broken template.
	err = os.WriteFile(filepath.Join(dir, "resilient.html.tmpl"), []byte(`{{.Broken`), 0o644)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Engine should still serve the original.
	r, err := engine.Render("resilient.html.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>Original</p>", r)

	cancel()
	assert.NoError(t, <-done)
}
