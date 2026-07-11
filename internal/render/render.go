package render

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Renderer struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

func New(fsys fs.FS, layouts []string, adminLayouts []string, pages []string) (*Renderer, error) {
	r := &Renderer{
		templates: make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"rub":            FormatKop,
			"kop2rub":        Kop2Rub,
			"kop2rubPtr":     Kop2RubPtr,
			"dict":           dict,
			"formatDate":     formatDate,
			"formatDateTime": formatDateTime,
			"formatDatePtr":  formatDatePtr,
			"weekday":        weekdayName,
			"add":            func(a, b int) int { return a + b },
			"pct": func(val, max int64) int {
				if max == 0 {
					return 0
				}
				p := int(val * 100 / max)
				if p < 4 {
					return 4
				}
				return p
			},
		},
	}

	for _, page := range pages {
		name := pageName(page)
		tmpl := template.New("").Funcs(r.funcMap)

		useLayouts := layouts
		if strings.HasPrefix(name, "admin_") && name != "admin_login" {
			useLayouts = adminLayouts
		} else if name == "admin_login" {
			useLayouts = nil
		}

		for _, l := range useLayouts {
			data, err := fs.ReadFile(fsys, l)
			if err != nil {
				return nil, fmt.Errorf("read layout %s: %w", l, err)
			}
			if _, err := tmpl.Parse(string(data)); err != nil {
				return nil, fmt.Errorf("parse layout %s: %w", l, err)
			}
		}

		data, err := fs.ReadFile(fsys, page)
		if err != nil {
			return nil, fmt.Errorf("read page %s: %w", page, err)
		}
		if _, err := tmpl.Parse(string(data)); err != nil {
			return nil, fmt.Errorf("parse page %s: %w", page, err)
		}

		r.templates[name] = tmpl
	}

	return r, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data any) error {
	tmpl, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "layout", data)
}

func pageName(path string) string {
	base := filepath.Base(path)
	return base[:len(base)-len(filepath.Ext(base))]
}

func FormatKop(kop int64) string {
	rub := kop / 100
	return fmt.Sprintf("%d ₽", rub)
}

func Kop2Rub(kop int64) string {
	rub := float64(kop) / 100.0
	return strconv.FormatFloat(rub, 'f', -1, 64)
}

func Kop2RubPtr(kop *int64) string {
	if kop == nil {
		return ""
	}
	return Kop2Rub(*kop)
}

func dict(values ...any) map[string]any {
	m := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key := fmt.Sprintf("%v", values[i])
		if i+1 < len(values) {
			m[key] = values[i+1]
		}
	}
	return m
}

func formatDate(t time.Time) string {
	return t.Format("02.01.2006")
}

func formatDateTime(t time.Time) string {
	return t.Format("02.01.2006 15:04")
}

func weekdayName(d int) string {
	names := [...]string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота", "Воскресенье"}
	if d < 1 || d > 7 {
		return "—"
	}
	return names[d-1]
}

func formatDatePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format("02.01.2006")
}
