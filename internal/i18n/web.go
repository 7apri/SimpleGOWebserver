package i18n

import (
	"context"
	"net/http"
	"time"
)

func (mgr *I18nManager) isSupported(l string) bool {
	s := mgr.snapshot.Load()
	if s == nil {
		return false
	}

	_, ok := s.Buckets[l]

	return ok
}

func (mgr *I18nManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Language")

		lang := r.URL.Query().Get("lang")

		if lang != "" {
			if !mgr.isSupported(lang) {
				return
			}

			if util.GetUserAgent(r).Bot {
				ctx := context.WithValue(r.Context(), LangKey, lang)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			c, err := r.Cookie("lang")

			if err != nil || c.Value != lang {
				http.SetCookie(w, &http.Cookie{
					Name:     "lang",
					Value:    lang,
					Path:     "/",
					MaxAge:   int((24 * time.Hour).Seconds() * 365),
					HttpOnly: false,
					Secure:   true,
					SameSite: http.SameSiteLaxMode,
				})

			}

			return

		}

		lang = GetLangFromReq(r)

		ctx := context.WithValue(r.Context(), LangKey, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

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
