package template

import (
	"html/template"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func setupTemplateDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}
	return dir
}

func TestNewEngine_LoadsTemplates(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"test.html.tmpl": `<h1>Hello, {{.Name}}!</h1>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)
	require.NotNil(t, engine)

	t.Logf("engine created: dir=%s", dir)
}

func TestNewEngine_DirNotFound(t *testing.T) {
	_, err := NewEngine("/nonexistent/path", nil, testLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template dir")

	t.Logf("error: %v", err)
}

func TestNewEngine_NotADirectory(t *testing.T) {
	f, err := os.CreateTemp("", "notadir")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.Close()

	_, err = NewEngine(f.Name(), nil, testLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestNewEngine_NoTemplateFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := NewEngine(dir, nil, testLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no")
}

func TestNewEngine_InvalidTemplate(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"bad.html.tmpl": `{{.Invalid`,
	})

	_, err := NewEngine(dir, nil, testLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template parse")
}

func TestEngine_Render_HappyPath(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"greeting.html.tmpl": `<p>Hello, {{.Name}}!</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	result, err := engine.Render("greeting.html.tmpl", map[string]string{"Name": "World"})
	require.NoError(t, err)
	assert.Equal(t, "<p>Hello, World!</p>", result)

	t.Logf("rendered: %s", result)
}

func TestEngine_Render_WithFuncMap(t *testing.T) {
	funcs := template.FuncMap{
		"upper": func(s string) string {
			result := make([]byte, len(s))
			for i, b := range []byte(s) {
				if b >= 'a' && b <= 'z' {
					result[i] = b - 32
				} else {
					result[i] = b
				}
			}
			return string(result)
		},
	}

	dir := setupTemplateDir(t, map[string]string{
		"upper.html.tmpl": `<p>{{upper .Name}}</p>`,
	})

	engine, err := NewEngine(dir, funcs, testLogger())
	require.NoError(t, err)

	result, err := engine.Render("upper.html.tmpl", map[string]string{"Name": "hello"})
	require.NoError(t, err)
	assert.Equal(t, "<p>HELLO</p>", result)
}

func TestEngine_Render_TemplateNotFound(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"exists.html.tmpl": `<p>OK</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	_, err = engine.Render("nonexistent.html.tmpl", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template render")
}

func TestEngine_Render_HTMLEscaping(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"escape.html.tmpl": `<p>{{.Input}}</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	result, err := engine.Render("escape.html.tmpl", map[string]string{"Input": `<script>alert("xss")</script>`})
	require.NoError(t, err)
	assert.NotContains(t, result, "<script>")
	assert.Contains(t, result, "&lt;script&gt;")

	t.Logf("escaped: %s", result)
}

func TestEngine_Render_MultipleTemplates(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"one.html.tmpl": `<p>One: {{.V}}</p>`,
		"two.html.tmpl": `<p>Two: {{.V}}</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	r1, err := engine.Render("one.html.tmpl", map[string]string{"V": "a"})
	require.NoError(t, err)
	assert.Equal(t, "<p>One: a</p>", r1)

	r2, err := engine.Render("two.html.tmpl", map[string]string{"V": "b"})
	require.NoError(t, err)
	assert.Equal(t, "<p>Two: b</p>", r2)
}

func TestEngine_Reload_HappyPath(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"dynamic.html.tmpl": `<p>Version 1</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	// Render version 1.
	r1, err := engine.Render("dynamic.html.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>Version 1</p>", r1)

	// Update the template file.
	err = os.WriteFile(filepath.Join(dir, "dynamic.html.tmpl"), []byte(`<p>Version 2</p>`), 0o644)
	require.NoError(t, err)

	// Reload.
	err = engine.Reload()
	require.NoError(t, err)

	// Render version 2.
	r2, err := engine.Render("dynamic.html.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>Version 2</p>", r2)
}

func TestEngine_Reload_FailSafe(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"safe.html.tmpl": `<p>Original</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	// Render original.
	r1, err := engine.Render("safe.html.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>Original</p>", r1)

	// Break the template.
	err = os.WriteFile(filepath.Join(dir, "safe.html.tmpl"), []byte(`{{.Broken`), 0o644)
	require.NoError(t, err)

	// Reload should fail.
	err = engine.Reload()
	require.Error(t, err)

	// Original template still works.
	r2, err := engine.Render("safe.html.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "<p>Original</p>", r2)

	t.Logf("fail-safe: reload error=%v, original still works", err)
}

func TestEngine_Render_ConcurrentAccess(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"concurrent.html.tmpl": `<p>{{.N}}</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	const goroutines = 50
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := engine.Render("concurrent.html.tmpl", map[string]int{"N": n})
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent render error: %v", err)
	}
}

func TestEngine_Reload_ConcurrentWithRender(t *testing.T) {
	dir := setupTemplateDir(t, map[string]string{
		"race.html.tmpl": `<p>{{.V}}</p>`,
	})

	engine, err := NewEngine(dir, nil, testLogger())
	require.NoError(t, err)

	const iterations = 100
	var wg sync.WaitGroup

	// Concurrent renders.
	for range iterations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = engine.Render("race.html.tmpl", map[string]string{"V": "test"})
		}()
	}

	// Concurrent reloads.
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = engine.Reload()
		}()
	}

	wg.Wait()
}
