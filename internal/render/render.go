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

// Renderer кэширует шаблоны, распарсенные один раз на старте.
type Renderer struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

// New парсит все шаблоны из fsys.
// layouts — для публичных страниц, adminLayouts — для админки.
// Страницы с именем, начинающимся на "admin_" (кроме "admin_login"),
// используют adminLayouts. admin_login — standalone (без layout).
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

		// Выбираем layout.
		useLayouts := layouts
		if strings.HasPrefix(name, "admin_") && name != "admin_login" {
			useLayouts = adminLayouts
		} else if name == "admin_login" {
			useLayouts = nil // standalone
		}

		// Парсим layout(s), затем страницу.
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

// Render выполняет шаблон и пишет в w.
func (r *Renderer) Render(w http.ResponseWriter, name string, data any) error {
	tmpl, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "layout", data)
}

// pageName извлекает имя страницы из пути (без расширения).
func pageName(path string) string {
	base := filepath.Base(path)
	return base[:len(base)-len(filepath.Ext(base))]
}

// FormatKop переводит копейки в строку вида "8 000 ₽".
func FormatKop(kop int64) string {
	rub := kop / 100
	return fmt.Sprintf("%d ₽", rub)
}

// Kop2Rub переводит копейки в рубли для input-полей форм: "8000", "800.5", "80.05".
func Kop2Rub(kop int64) string {
	rub := float64(kop) / 100.0
	return strconv.FormatFloat(rub, 'f', -1, 64)
}

// Kop2RubPtr — как Kop2Rub, но для *int64: nil возвращает "".
func Kop2RubPtr(kop *int64) string {
	if kop == nil {
		return ""
	}
	return Kop2Rub(*kop)
}

// dict — хелпер для создания map в шаблонах.
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

// formatDate форматирует time.Time в "02.01.2006".
func formatDate(t time.Time) string {
	return t.Format("02.01.2006")
}

// formatDateTime форматирует time.Time в "02.01.2006 15:04".
func formatDateTime(t time.Time) string {
	return t.Format("02.01.2006 15:04")
}

// weekdayName переводит ISO-номер дня недели (1=Пн … 7=Вс) в русское название.
func weekdayName(d int) string {
	names := [...]string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота", "Воскресенье"}
	if d < 1 || d > 7 {
		return "—"
	}
	return names[d-1]
}

// formatDatePtr форматирует *time.Time в "02.01.2006", для nil возвращает "—".
func formatDatePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format("02.01.2006")
}
