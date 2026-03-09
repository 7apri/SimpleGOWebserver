package templates

import (
	"net/http"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

func cleanETag(s string) string {
	if len(s) >= 2 && s[0:2] == "W/" {
		s = s[2:]
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

func (mgr *TemplateManager) RenderPage(w http.ResponseWriter, r *http.Request, key TemplateKey, data any) *web.WebError {
	lang := i18n.GetLangFromReq(r)
	tmpl := mgr.Get(lang, key)

	etagClient := r.Header.Get("If-None-Match")

	if etagClient == "" {
		if c, err := r.Cookie("X-Version"); err == nil {
			etagClient = c.Value
		}
	}
	etagClient = cleanETag(etagClient)

	if etagClient == tmpl.Etag {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}

	w.Header().Set("ETag", `W/"`+tmpl.Etag+`"`)
	w.Header().Set("Cache-Control", "no-cache")

	http.SetCookie(w, &http.Cookie{
		Name:     "X-Version",
		Value:    tmpl.Etag,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	if err := tmpl.Execute(w, data); err != nil {
		return web.NewError(http.StatusInternalServerError, "err_internal", err, nil)
	}

	return nil
}

func (mgr *TemplateManager) Get(lang string, key TemplateKey) *TemplateWrapper {
	s := mgr.snapshot.Load()
	if s == nil {
		return nil
	}

	if l, ok := s.templates[lang]; ok {
		if val, ok := l[key]; ok {
			return val
		}
	}
	if l, ok := s.templates["en"]; ok {
		if val, ok := l[key]; ok {
			return val
		}
	}

	return nil
}

type metadata map[string]string

func getMetadata(content string) (metadata, string) {
	metadata := make(map[string]string)

	if !strings.HasPrefix(content, "---") {
		return metadata, content
	}

	endIndex := strings.Index(content[3:], "---")
	if endIndex == -1 {
		return metadata, content
	}

	endIndex += 3

	frontmatter := content[3:endIndex]
	remaining := content[endIndex+3:]

	lines := strings.SplitSeq(strings.ReplaceAll(frontmatter, "\r\n", "\n"), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		val = strings.Trim(val, `"'`)
		metadata[key] = val
	}

	return metadata, strings.TrimSpace(remaining)
}
