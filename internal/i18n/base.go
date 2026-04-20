package i18n

import (
	"context"
	"crypto/rand"
	"errors"
	"maps"
	"math/big"
	randInsecure "math/rand/v2"
	"net/http"
	"strings"
)

type contextKey string

const LangKey contextKey = "lang"

func GetLangFromContext(ctx context.Context) (string, bool) {
	lang, ok := ctx.Value(LangKey).(string)
	return lang, ok
}
func GetLangFromReq(r *http.Request) string {
	lang, ok := GetLangFromContext(r.Context())
	if ok {
		return lang
	}

	lang = "en"

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

func (m *I18nManager) TranslateError(lang, key string) (string, error) {
	s := m.snapshot.Load()
	if s == nil {
		return "", ErrorNoSnapshot
	}
	if bucket, ok := s.Buckets[lang]; ok {
		if val, ok := bucket.Errors[key]; ok {
			return val, nil
		}
	}
	if lang != "en" {
		if enBucket, ok := s.Buckets["en"]; ok {
			if val, ok := enBucket.Errors[key]; ok {
				return val, nil
			}
		}
	}

	return key, ErrorKeyNotFound
}

func (mgr *I18nManager) GetClient(lang string, scripts map[string]string) map[string]string {
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
func (mgr *I18nManager) GetBank(lang string) ([]string, error) {
	s := mgr.snapshot.Load()
	if s == nil {
		return nil, ErrorNoSnapshot
	}

	bank, ok := s.WordBanks[lang]
	if !ok && lang != "en" {
		bank = s.WordBanks["en"]
	}
	if bank == nil {
		return nil, ErrorKeyNotFound
	}

	return bank, nil
}
func (mgr *I18nManager) GetUsernameFormats(lang string) ([]string, error) {
	s := mgr.snapshot.Load()
	if s == nil {
		return nil, ErrorNoSnapshot
	}

	formats, ok := s.UsernameFormats[lang]
	if !ok && lang != "en" {
		formats = s.UsernameFormats["en"]
	}
	if formats == nil {
		return nil, ErrorKeyNotFound
	}

	return formats, nil
}
func (mgr *I18nManager) GetUsernames(lang, base string, count int) ([]string, error) {
	formats, err := mgr.GetUsernameFormats(lang)
	if err != nil {
		return nil, err
	}
	if count > len(formats) {
		count = len(formats)
	}

	idxs := randInsecure.Perm(len(formats))

	usernames := make([]string, 0, count)
	for i := 0; i < count; i++ {
		format := formats[idxs[i]]
		usernames = append(usernames, strings.ReplaceAll(format, "%s", base))
	}

	return usernames, nil
}
func (mgr *I18nManager) PickWords(lang string, count int) ([]string, error) {
	list, err := mgr.GetBank(lang)
	if err != nil {
		return nil, err
	}

	bigN := big.NewInt(int64(len(list)))
	selected := make([]string, count)

	for i := range count {
		idx, err := rand.Int(rand.Reader, bigN)
		if err != nil {
			return nil, err
		}
		selected[i] = list[idx.Int64()]
	}

	return selected, nil
}
