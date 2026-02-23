package templates

import (
	"html/template"
	"io"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
)

type TemplateWrapper struct {
	t *template.Template
	e string
}

func (w *TemplateWrapper) Execute(wr io.Writer, data any) error {
	if w.e != "" {
		return w.t.ExecuteTemplate(wr, w.e, data)
	}
	return w.t.Execute(wr, data)
}

func (mgr *Manager) RenderPage(w io.Writer, r *http.Request, name string, data any) error {
	lang := i18n.GetLangFromReq(r)
	tmpl := mgr.Get(lang, "page", name)
	return tmpl.Execute(w, data)
}

func (mgr *Manager) Get(lang, kind, name string) *TemplateWrapper {
	s := mgr.snapshot.Load()
	if s == nil {
		return nil
	}

	key := templateKey{kind: kind, name: name}

	if l, ok := s.templates[lang]; ok {
		if val, ok := l[key]; ok {
			return val
		}
	}
	if l, ok := s.templates["en"]; ok {
		return l[key]
	}

	return nil
}
