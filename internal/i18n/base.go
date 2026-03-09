package i18n

import (
	"context"
	"errors"
	"maps"
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

var ErrorNoSnapshot = errors.New("i18n no spanshot")
var ErrorKeyNotFound = errors.New("i18n key not found")

func (m *I18nManager) Translate(lang, key string) (string, error) {
	s := m.snapshot.Load()
	if s == nil {
		return "", ErrorNoSnapshot
	}
	if bucket, ok := s.Buckets[lang]; ok {
		if val, ok := bucket.Static[key]; ok {
			return val, nil
		}
	}
	if lang != "en" {
		if enBucket, ok := s.Buckets["en"]; ok {
			if val, ok := enBucket.Static[key]; ok {
				return val, nil
			}
		}
	}

	return key, ErrorKeyNotFound
}

func (mgr *I18nManager) GetClient(lang string, scripts map[string]struct{}) map[string]string {
	s := mgr.snapshot.Load()
	if s == nil {
		return nil
	}

	bucket, ok := s.Buckets[lang]
	if !ok && lang != "en" {
		bucket = s.Buckets["en"]
	}
	if bucket == nil {
		return make(map[string]string)
	}

	result := make(map[string]string)

	if global, exists := bucket.Client["_global"]; exists {
		maps.Copy(result, global)
	}

	for scriptName := range scripts {
		if scriptKeys, exists := bucket.Client[scriptName]; exists {
			maps.Copy(result, scriptKeys)
		}
	}

	return result
}
