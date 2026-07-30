package handlers

import (
	"os"
	"testing"

	"therunish/internal/render"
)

const templatesDir = "../../web/templates"

// TestTemplatesParse ловит опечатки в шаблонах: все страницы из web/templates
// должны собираться со своими лейаутами.
func TestTemplatesParse(t *testing.T) {
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		t.Fatal(err)
	}

	layouts := map[string]bool{"admin_layout.html": true, "login_layout.html": true}
	var pages []string
	for _, e := range entries {
		if !layouts[e.Name()] {
			pages = append(pages, e.Name())
		}
	}

	if _, err := render.New(
		os.DirFS(templatesDir),
		[]string{"login_layout.html"},
		[]string{"admin_layout.html"},
		pages,
	); err != nil {
		t.Fatalf("parse templates: %v", err)
	}
}
