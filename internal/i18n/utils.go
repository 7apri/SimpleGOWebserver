package i18n

import (
	"context"
	"html/template"
	"net/http"
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
		lang = "en"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   31536000,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
}

func (m *Manager) GetAvailableLangs() []string {
	return m.langs
}
func (m *Manager) GetWeather() map[string]map[string]string {
	return m.apiWeatherT
}
func (m *Manager) GetWeatherT() map[string]map[int16]string {
	result := make(map[string]map[int16]string, len(m.langs))

	for _, lang := range m.langs {
		result[lang] = make(map[int16]string, len(WeatherIDToKey))
		for id, key := range WeatherIDToKey {
			result[lang][id] = m.apiWeatherT[lang][key]
		}
	}
	return result
}
func (m *Manager) GetStatic(lang, key string) string {
	return m.pageAllT[lang][key]
}
func (m *Manager) GetDynamicJSON(lang, page string) template.JS {
	return m.pageDynamicT[lang][page]
}

func (m *Manager) Get(lang, key string) string {
	if t, ok := m.pageAllT[lang][key]; ok {
		return t
	}
	if t, ok := m.pageAllT["en"][key]; ok {
		return t
	}
	return "no i18n for: " + key + "#!"
}
func (m *Manager) GetJSON(lang, page string) template.JS {
	if data, ok := m.pageDynamicT[lang][page]; ok {
		return data
	}
	return template.JS("{}")
}
