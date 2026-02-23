package i18n

import (
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
)

type Lang struct {
	Code           string
	SwitchSentence string
}

type i18nSnapshot struct {
	pageAllT     map[string]map[string]string      // [lang][page]
	pageDynamicT map[string]map[string]template.JS // [lang][page]
	apiWeatherT  map[string]map[int16]string       // [lang][weather_id]
	langs        []Lang
}
type Manager struct {
	RefreshChan chan struct{}
	i18nFS      fs.FS
	snapshot    atomic.Pointer[i18nSnapshot]
}

func NewManager(i18nFS fs.FS) (*Manager, error) {
	mgr := &Manager{
		i18nFS:      i18nFS,
		RefreshChan: make(chan struct{}, 1),
	}
	if err := mgr.Refresh(); err != nil {
		return nil, err
	}
	<-mgr.RefreshChan
	mgr.Watcher(time.Second)
	return mgr, nil
}
func (mgr *Manager) Watcher(delay time.Duration) {
	go func() {
		var lastHighestMod time.Time
		for {
			time.Sleep(delay)

			currentHighest := lastHighestMod

			err := fs.WalkDir(mgr.i18nFS, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
					return nil
				}

				info, _ := d.Info()
				if info.ModTime().After(currentHighest) {
					currentHighest = info.ModTime()
				}
				return nil
			})

			if err != nil {
				continue
			}

			if currentHighest.After(lastHighestMod) {
				if !lastHighestMod.IsZero() {
					slog.Info("Translation file change detected")
					mgr.Refresh()
				}
				lastHighestMod = currentHighest
			}
		}
	}()
}
func (mgr *Manager) Refresh() error {
	files, err := fs.ReadDir(mgr.i18nFS, ".")
	if err != nil {
		return err
	}
	weatherIDToKey := getWeatherIdToKey()

	newSnapshot := i18nSnapshot{
		pageAllT:     make(map[string]map[string]string, len(files)),
		pageDynamicT: make(map[string]map[string]template.JS, len(files)),
		apiWeatherT:  make(map[string]map[int16]string, len(files)),
		langs:        make([]Lang, 0, len(files)),
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}

		lang := strings.TrimSuffix(f.Name(), ".json")

		data, err := fs.ReadFile(mgr.i18nFS, f.Name())
		if err != nil {
			slog.Error("Failed to read", "file", f.Name(), "error", err)
			continue
		}

		var raw struct {
			SwitchSentence string                       `json:"switch_lang"`
			PageStatic     map[string]string            `json:"page_static"`
			PageDynamic    map[string]map[string]string `json:"page_dynamic"`
			ApiWeather     map[string]string            `json:"api_weather"`
		}
		if err := sonic.Unmarshal(data, &raw); err != nil {
			slog.Error("Failed to unmarshal JSON", "file", f.Name(), "error", err)
			continue
		}

		slog.Info("processing translations", "lang", lang)

		newSnapshot.langs = append(newSnapshot.langs, Lang{
			Code:           lang,
			SwitchSentence: raw.SwitchSentence,
		})

		newSnapshot.apiWeatherT[lang] = make(map[int16]string, len(weatherIDToKey))
		for id, key := range weatherIDToKey {
			if trans, ok := raw.ApiWeather[key]; ok {
				newSnapshot.apiWeatherT[lang][id] = trans
			}
		}

		totalPageKeys := len(raw.PageStatic)
		for _, pageMap := range raw.PageDynamic {
			totalPageKeys += len(pageMap)
		}

		newSnapshot.pageAllT[lang] = make(map[string]string, totalPageKeys)
		newSnapshot.pageDynamicT[lang] = make(map[string]template.JS, len(raw.PageDynamic))

		dynamic_GlobalMap := make(map[string]string)
		if global, ok := raw.PageDynamic["_global"]; ok {
			for k, v := range global {
				dynamic_GlobalMap[k] = v
				newSnapshot.pageAllT[lang][k] = v
			}
			delete(raw.PageDynamic, "_global")
		}

		for key, t := range raw.PageDynamic {
			mergedPage := make(map[string]string, len(dynamic_GlobalMap)+len(t))
			maps.Copy(mergedPage, dynamic_GlobalMap)

			for k, v := range t {
				mergedPage[k] = v
				newSnapshot.pageAllT[lang][k] = v
			}

			jsonBytes, err := sonic.Marshal(mergedPage)
			if err != nil {
				slog.Error("Failed to marshal dynamic", "file", f.Name(), "page", key, "error", err)
				continue
			}

			for page := range strings.SplitSeq(key, ",") {
				newSnapshot.pageDynamicT[lang][strings.TrimSpace(page)] = template.JS(jsonBytes)
			}
		}

		maps.Copy(newSnapshot.pageAllT[lang], raw.PageStatic)
	}

	mgr.RefreshChan <- struct{}{}
	mgr.snapshot.Store(&newSnapshot)

	totalPagesDynamic := 0
	for _, l := range newSnapshot.pageDynamicT {
		totalPagesDynamic += len(l)
	}
	slog.Info("i18n_refresh_complete",
		slog.Group("metrics",
			slog.Int("langs", len(newSnapshot.langs)),
			slog.Int("pages_dynamic", totalPagesDynamic),
		),
	)
	return nil
}
