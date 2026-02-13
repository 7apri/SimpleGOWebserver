package i18n

import (
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"strings"

	"github.com/bytedance/sonic"
)

type Manager struct {
	pageAllT     map[string]map[string]string      // [lang][page]
	pageDynamicT map[string]map[string]template.JS // [lang][page]
	apiWeatherT  map[string]map[string]string      // [lang][key]
	langs        []string
}

func NewManager(i18nDir fs.FS) (*Manager, error) {
	files, err := fs.ReadDir(i18nDir, ".")
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		pageAllT:     make(map[string]map[string]string, len(files)),
		pageDynamicT: make(map[string]map[string]template.JS, len(files)),
		apiWeatherT:  make(map[string]map[string]string, len(files)),
		langs:        make([]string, 0, len(files)),
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}

		lang := strings.TrimSuffix(f.Name(), ".json")
		mgr.langs = append(mgr.langs, lang)

		var data []byte
		var err error
		data, err = fs.ReadFile(i18nDir, f.Name())
		if err != nil {
			slog.Error("Failed to read", "file", f.Name(), "error", err)
			continue
		}

		var raw struct {
			PageStatic  map[string]string            `json:"page_static"`
			PageDynamic map[string]map[string]string `json:"page_dynamic"`
			ApiWeather  map[string]string            `json:"api_weather"`
		}
		if err := sonic.Unmarshal(data, &raw); err != nil {
			slog.Error("Failed to unmarshal JSON", "file", f.Name(), "error", err)
			continue
		}

		slog.Info("adding translations", "lang", lang)

		mgr.apiWeatherT[lang] = make(map[string]string, len(raw.ApiWeather))
		maps.Copy(mgr.apiWeatherT[lang], raw.ApiWeather)

		totalPageKeys := len(raw.PageStatic)
		for _, pageMap := range raw.PageDynamic {
			totalPageKeys += len(pageMap)
		}

		mgr.pageAllT[lang] = make(map[string]string, totalPageKeys)
		mgr.pageDynamicT[lang] = make(map[string]template.JS, len(raw.PageDynamic))

		dynamic_GlobalMap := make(map[string]string)
		if global, ok := raw.PageDynamic["_global"]; ok {
			for k, v := range global {
				dynamic_GlobalMap[k] = v
				mgr.pageAllT[lang][k] = v
			}
			delete(raw.PageDynamic, "_global")
		}

		for key, t := range raw.PageDynamic {
			mergedPage := make(map[string]string, len(dynamic_GlobalMap)+len(t))
			maps.Copy(mergedPage, dynamic_GlobalMap)

			for k, v := range t {
				mergedPage[k] = v
				mgr.pageAllT[lang][k] = v
			}

			jsonBytes, err := sonic.Marshal(mergedPage)
			if err != nil {
				slog.Error("Failed to marshal dynamic translations", "file", f.Name(), "page", key, "error", err)
				continue
			}

			for page := range strings.SplitSeq(key, ",") {
				mgr.pageDynamicT[lang][strings.TrimSpace(page)] = template.JS(jsonBytes)
			}
		}

		maps.Copy(mgr.pageAllT[lang], raw.PageStatic)
	}
	return mgr, nil
}
