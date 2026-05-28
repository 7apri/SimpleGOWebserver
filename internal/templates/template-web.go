package templates

import (
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/cespare/xxhash/v2"
)

func xxHashETag(base string, meta ...string) string {
	h := xxhash.New()
	_, _ = h.WriteString(base)
	for _, part := range meta {
		_, _ = h.WriteString(part)
	}
	return strconv.FormatUint(h.Sum64(), 36)
}
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
	fullTag := `W/"` + tag + `"`

	w.Header().Set("ETag", fullTag)
	w.Header().Set("Cache-Control", "no-cache")

	cc := r.Header.Get("Cache-Control")
	if strings.Contains(cc, "no-cache") || strings.Contains(cc, "no-store") {
		return false
	}

	etagClient := r.Header.Get("If-None-Match")
	if etagClient != "" && cleanETag(etagClient) == tag {
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	return false
}

func (mgr *TemplateManager) WriteTemplateETag(w http.ResponseWriter, r *http.Request, key TemplateKey, meta string, data any) *web.WebError {
	lang := i18n.GetLangFromReq(r)

	tmpl := mgr.Get(lang, key)
	if tmpl == nil {
		return web.NewError(http.StatusNotFound, "not_found", nil, key)
	}

	if SetETag(w, r, tmpl.Etag+meta) {
		return nil
	}

	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)

	if err := tmpl.Execute(b, data); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, key)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(b.Len()))
	_, err := w.Write(b.Bytes())
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "write_failed", err, nil)
	}

	return nil
}

func (mgr *TemplateManager) WriteTemplateSpecific(w http.ResponseWriter, tmpl *TemplateWrapper, data any) *web.WebError {
	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)

	if err := tmpl.Execute(b, data); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(b.Len()))
	_, err := w.Write(b.Bytes())
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "write_failed", err, nil)
	}

	return nil
}

func (mgr *TemplateManager) WriteTemplate(w io.Writer, lang string, key TemplateKey, data any) error {
	tmpl := mgr.Get(lang, key)
	if tmpl == nil {
		return i18n.ErrorKeyNotFound
	}

	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)

	if err := tmpl.Execute(b, data); err != nil {
		return err
	}

	_, err := w.Write(b.Bytes())
	if err != nil {
		return err
	}

	return nil
}

type HtmxBodyPageData struct {
	Body template.HTML
	Data any
}

func (mgr *TemplateManager) WriteTemplateHtmx(w http.ResponseWriter, r *http.Request, body TemplateKey, fragment TemplateKey, data any, meta ...string) *web.WebError {
	lang := i18n.GetLangFromReq(r)
	w.Header().Add("Vary", "HX-Request")

	tmplFragment := mgr.Get(lang, fragment)
	if tmplFragment == nil {
		return web.NewError(http.StatusNotFound, "not_found", nil, fragment)
	}
	etag := tmplFragment.Etag

	var tmplBody *TemplateWrapper
	isFullReq := r.Header.Get("HX-Request") == ""

	if isFullReq {
		tmplBody = mgr.Get(lang, body)
		if tmplBody == nil {
			return web.NewError(http.StatusNotFound, "not_found", nil, body)
		}

		etag += tmplBody.Etag
	}
	if SetETag(w, r, xxHashETag(etag, meta...)) {
		return nil
	}

	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)

	if err := tmplFragment.Execute(b, data); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, fragment)
	}
	if isFullReq {
		body := template.HTML(b.String())
		b.Reset()
		if err := tmplBody.Execute(b, HtmxBodyPageData{
			Body: body,
			Data: data,
		}); err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, tmplBody)
		}
	}

	w.Header().Set("Content-Length", strconv.Itoa(b.Len()))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write(b.Bytes())
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "write_failed", err, nil)
	}

	return nil
}
