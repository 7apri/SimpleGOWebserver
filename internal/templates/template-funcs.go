package templates

import (
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/bytedance/sonic"
)

const brand = "Panels"

func (mgr *TemplateManager) funcMapExec(lang i18n.Lang) template.FuncMap {
	m := template.FuncMap{
		"tr": func(key string) string {
			val, _ := mgr.i18nManager.Translate(lang.Code, key)
			return val
		},
		"asset": mgr.getAsset,
	}
	maps.Copy(m, mgr.funcMapBase())
	return m
}
func (mgr *TemplateManager) funcMapBase() template.FuncMap {
	return template.FuncMap{
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call: must have even number of arguments")
			}
			dict := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"slice": func(args ...any) []any {
			return args
		},
		"seq": func(n int) []struct{} {
			return make([]struct{}, n)
		},
		"printy": func(from string, data ...any) string {
			slog.Info("printy",
				slog.Group("params",
					slog.String("from", from),
					slog.Any("data", data),
				),
			)
			return ""
		},
		"concat": func(strs ...string) string {
			if len(strs) == 2 {
				return strs[0] + strs[1]
			}
			return strings.Join(strs, "")
		},
		"capitalize": func(s string) string {
			if len(s) == 0 {
				return ""
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
	}
}

func (mgr *TemplateManager) funcMapBake(e *bakeEnv) template.FuncMap {
	funcs := template.FuncMap{
		"tr": func(key string) string {
			if e.isCtxNil() {
				return key
			}
			val, _ := mgr.i18nManager.Translate(e.ctx.Lang.Code, key)
			return val
		},
		"getTitleBrand": func() string {
			if e.isCtxNil() {
				return brand
			}
			title, err := mgr.i18nManager.Translate(e.ctx.Lang.Code, fmt.Sprintf("title:%s", e.ctx.Name))
			if err != nil || title == "" {
				title = brand
			} else {
				title = fmt.Sprintf("%s | %s", title, brand)
			}
			return title
		},
		"getTitle": func() string {
			if e.isCtxNil() {
				return brand
			}
			title, _ := mgr.i18nManager.Translate(e.ctx.Lang.Code, fmt.Sprintf("title:%s", e.ctx.Name))
			return title
		},
		"getDesc": func() string {
			if e.isCtxNil() {
				return ""
			}
			desc, _ := mgr.i18nManager.Translate(e.ctx.Lang.Code, fmt.Sprintf("desc:%s", e.ctx.Name))
			return desc
		},
		"getTranslations": func() template.JS {
			if e.isCtxNil() {
				return "{}"
			}
			m := mgr.i18nManager.GetClient(e.ctx.Lang.Code, e.ctx.Scripts)
			b, err := sonic.Marshal(m)
			if err != nil {
				slog.Error("Failed to marshal client translations", "error", err)
				return "{}"
			}
			return template.JS(b)
		},
		"import": func(path string, data any) (string, error) {
			if e.isCtxNil() || e.files.IsNil() {
				return "", nil
			}
			if e.ctx.Depth > 10 {
				return "", fmt.Errorf("max import depth exceeded at %s", path)
			}
			targetPath, target := e.getTargetPath(path, "components/")
			if targetPath == "" {
				return "", fmt.Errorf("import not found: %s", path)
			}
			if e.ctx.Meta["path"] == targetPath {
				return "", fmt.Errorf("cant import itself: %s", targetPath)
			}

			childCtx := bakeContextPool.Get().(*BakeContext)
			defer bakeContextPool.Put(childCtx)
			childCtx.Reset()

			e.ctx.SubContextInto(childCtx)
			childCtx.InsertMeta(target.Meta)

			if data != nil {
				childCtx.Data = data
			} else {
				childCtx.Data = make(map[string]any)
			}

			return mgr.bake(target, e.withCtx(childCtx))
		},
		"registerScript": func(path string) string {
			if e.isCtxNil() {
				return ""
			}
			if !strings.HasSuffix(path, ".js") {
				path += ".js"
			}
			path, _ = strings.CutPrefix(path, "/")

			e.ctx.Scripts[path] = struct{}{}
			return ""
		},
		"asset": func(path string) string {
			hashes := mgr.assetHashes.Load()
			if hashes == nil {
				return path
			}
			target, _ := getTargetPath(path, "/static/", "", *hashes)
			return `{{ asset "` + target + `" }}`
		},
	}
	maps.Copy(funcs, mgr.funcMapBase())
	return funcs
}
