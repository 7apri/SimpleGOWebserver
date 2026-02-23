package i18n

import (
	"context"
	"errors"
	"html/template"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/pkg/util"
)

type contextKey string

const LangKey contextKey = "lang"

func GetLangFromContext(ctx context.Context) (string, bool) {
	lang, ok := ctx.Value(LangKey).(string)
	return lang, ok
}
func GetLangFromReq(r *http.Request) string {
	lang := "en"

	if cookie, err := r.Cookie("lang"); err == nil {
		lang = cookie.Value
	} else {
		accept := r.Header.Get("Accept-Language")
		if len(accept) >= 2 {
			lang = accept[:2]
		}
	}
	return lang
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
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusOK)
}
func (mgr *Manager) GetAvailableLangs() []string {
	s := mgr.snapshot.Load()
	langs := make([]string, 0, len(s.langs))
	for _, lang := range s.langs {
		langs = append(langs, lang.Code)
	}
	return langs
}
func (mgr *Manager) GetAvailableLangsWhole() []Lang {
	s := mgr.snapshot.Load()
	return s.langs
}

var ErrorNoSnapshot = errors.New("i18n no spanshot")
var ErrorKeyNotFound = errors.New("i18n key not found")

func (mgr *Manager) GetWeather(lang string, key int16) (string, error) {
	s := mgr.snapshot.Load()
	if s == nil {
		return "", ErrorNoSnapshot
	}

	if langMap, ok := s.apiWeatherT[lang]; ok {
		if val, ok := langMap[key]; ok {
			return val, nil
		}
	}

	if lang != "en" {
		if enMap, ok := s.apiWeatherT["en"]; ok {
			if val, ok := enMap[key]; ok {
				return val, nil
			}
		}
	}

	return "", ErrorKeyNotFound
}
func (mgr *Manager) InternalWeatherMap(lang string) (util.ReadOnlyMap[int16, string], error) {
	s := mgr.snapshot.Load()
	if s == nil {
		return util.ReadOnlyMap[int16, string]{}, ErrorNoSnapshot
	}

	if langMap, ok := s.apiWeatherT[lang]; ok {
		return util.NewReadOnlyMap(langMap), nil
	}

	return util.ReadOnlyMap[int16, string]{}, ErrorKeyNotFound
}

func (mgr *Manager) GetPageStatic(lang, key string) string {
	s := mgr.snapshot.Load()
	if s == nil {
		return key
	}

	if l, ok := s.pageAllT[lang]; ok {
		if val, ok := l[key]; ok {
			return val
		}
	}
	return key
}
func (mgr *Manager) GetPageDynamic(lang, key string) template.JS {
	s := mgr.snapshot.Load()
	if s == nil {
		return template.JS("{}")
	}

	if l, ok := s.pageDynamicT[lang]; ok {
		if val, ok := l[key]; ok {
			return val
		}
	}
	return template.JS("{}")
}
