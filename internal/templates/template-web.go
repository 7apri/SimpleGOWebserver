package templates

import (
	"io"
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

func SetETag(w http.ResponseWriter, r *http.Request, tag string) bool {
	cc := r.Header.Get("Cache-Control")
	cQ := r.URL.Query().Encode()
	isForceRefresh := strings.Contains(cc, "no-cache") || strings.Contains(cc, "no-store")

	if !isForceRefresh {
		etagClient := r.Header.Get("If-None-Match")
		if etagClient != "" {
			if cleanETag(etagClient) == tag {
				w.WriteHeader(http.StatusNotModified)
				return true
			}
		}
		var queryClient string
		if c, err := r.Cookie("X-Version"); err == nil {
			split := strings.Split(c.Value, "?")
			etagClient = split[0]
			queryClient = split[1]
		}
		if cleanETag(etagClient) == tag && cQ == queryClient {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}

	w.Header().Set("ETag", `W/"`+tag+`"`)
	w.Header().Set("Cache-Control", "no-cache")

	http.SetCookie(w, &http.Cookie{
		Name:     "X-Version",
		Value:    tag + "?" + cQ,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	return false
}

func (mgr *TemplateManager) WriteTemplateETag(w http.ResponseWriter, r *http.Request, key TemplateKey, data any) *web.WebError {
	lang := i18n.GetLangFromReq(r)

	tmpl := mgr.Get(lang, key)
	if tmpl == nil {
		return web.NewError(http.StatusNotFound, "err_not_found", nil, key)
	}

	if SetETag(w, r, tmpl.Etag) {
		return nil
	}

	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)

	if err := tmpl.Execute(b, data); err != nil {
		return web.NewError(http.StatusInternalServerError, "err_internal", err, key)
	}
	w.Write(b.Bytes())
	return nil
}

func (mgr *TemplateManager) WriteTemplateSpecific(w io.Writer, tmpl *TemplateWrapper, data any) *web.WebError {
	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)

	if err := tmpl.Execute(b, data); err != nil {
		return web.NewError(http.StatusInternalServerError, "err_internal", err, nil)
	}

	_, err := w.Write(b.Bytes())
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "err_write_failed", err, nil)
	}

	return nil
}

func (mgr *TemplateManager) WriteTemplate(w io.Writer, lang string, key TemplateKey, data any) *web.WebError {
	tmpl := mgr.Get(lang, key)
	if tmpl == nil {
		return web.NewError(http.StatusNotFound, "err_not_found", nil, key)
	}

	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)

	if err := tmpl.Execute(b, data); err != nil {
		return web.NewError(http.StatusInternalServerError, "err_internal", err, nil)
	}

	_, err := w.Write(b.Bytes())
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "err_write_failed", err, nil)
	}

	return nil
}
