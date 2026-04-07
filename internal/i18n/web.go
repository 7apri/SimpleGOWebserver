package i18n

import (
	"context"
	"net/http"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), LangKey, GetLangFromReq(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func HandleSetLang(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("l")
	if lang == "" {
		w.WriteHeader(http.StatusBadRequest)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   31536000,
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusOK)
}
func (mgr *I18nManager) GetAvailableLangs() []string {
	s := mgr.snapshot.Load()
	if s == nil {
		return nil
	}
	langs := make([]string, 0, len(s.Buckets))
	for _, bucket := range s.Buckets {
		langs = append(langs, bucket.Meta.Code)
	}
	return langs
}
func (mgr *I18nManager) GetAvailableLangsWhole() []Lang {
	s := mgr.snapshot.Load()
	if s == nil {
		return nil
	}
	langs := make([]Lang, 0, len(s.Buckets))
	for _, bucket := range s.Buckets {
		langs = append(langs, bucket.Meta)
	}
	return langs
}
