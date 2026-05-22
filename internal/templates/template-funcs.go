package templates

import (
	"fmt"
	"html/template"
	"log/slog"
	"maps"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/bytedance/sonic"
)

const brand = "Panels"

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
func (mgr *TemplateManager) funcMapExec(lang i18n.Lang) template.FuncMap {
	m := template.FuncMap{
		"tr": func(key string) string {
			val, _ := mgr.i18nManager.Translate(lang.Code, key)
			return val
		},
		"suggestions": func(base string, count int) []string {
			val, _ := mgr.i18nManager.GetUsernames(lang.Code, base, count)
			return val
		},
	}
	maps.Copy(m, mgr.funcMapBase())
	return m
}

func walk(name string, scriptType ScriptType, bucket *ScriptBucked, assetInfo *util.ReadOnlyMap[string, AssetInfo]) error {
	if script, seen := bucket.Resolved[name]; seen {
		if bucket.ScriptSeq[script].Type < scriptType {
			bucket.ScriptSeq[script].Type = scriptType
		}
		return nil
	}
	path := "/static/js/" + name
	if info, ok := assetInfo.Lookup(path); ok {
		for _, dep := range info.Deps {
			if err := walk(dep, ScriptTypePreload, bucket, assetInfo); err != nil {
				bucket.Resolved[name] = -1
				return err
			}
		}
		bucket.ScriptSeq = append(bucket.ScriptSeq, Script{
			HashedPath: path + "?v=" + info.Hash,
			Type:       scriptType,
		})
		bucket.Resolved[name] = len(bucket.ScriptSeq) - 1
	} else {
		return fmt.Errorf("script with the path %s was not found", path)
	}

	return nil
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
			m := mgr.i18nManager.GetClient(e.ctx.Lang.Code, e.ctx.Scripts.Resolved)
			b, err := sonic.ConfigStd.Marshal(m)
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

			return mgr.importTemplate(target, e, data)
		},
		"registerScript": func(path string) string {
			if e.isCtxNil() {
				return ""
			}
			assetInfo := mgr.assetInfo.Load()
			if assetInfo == nil {
				return ""
			}

			if !strings.HasSuffix(path, ".js") {
				path += ".js"
			}
			path, _ = strings.CutPrefix(path, "/")
			walk(path, ScriptTypeModule, e.ctx.Scripts, assetInfo)
			return ""
		},
		"registerScriptGlobal": func(path string) string {
			if e.isCtxNil() {
				return ""
			}
			assetInfo := mgr.assetInfo.Load()
			if assetInfo == nil {
				return ""
			}

			if !strings.HasSuffix(path, ".js") {
				path += ".js"
			}
			path, _ = strings.CutPrefix(path, "/")
			walk(path, ScriptTypeGlobal, e.ctx.Scripts, assetInfo)
			return ""
		},
		"registerDefine": func(name, content string) string {
			if e.isCtxNil() {
				return ""
			}
			e.ctx.Defines[name] = content
			return ""
		},
		"setMeta": func(key, value string) string {
			if e.isCtxNil() {
				return ""
			}
			e.ctx.Meta[key] = value
			return ""
		},
		"setData": func(value any) string {
			if e.isCtxNil() {
				return ""
			}
			e.ctx.Data = value
			return ""
		},
		"asset": mgr.getAsset,
		"getScriptImports": func() (template.HTML, error) {
			if e.isCtxNil() {
				return "", nil
			}

			b := mgr.bufferPool.Get()
			defer mgr.bufferPool.Put(b)

			imports := make(map[string]string)
			for name, n := range e.ctx.Scripts.Resolved {
				if n != -1 {
					imports["/static/js/"+name] = name
				}
			}

			if len(imports) != 0 {
				if m, err := sonic.ConfigStd.Marshal(map[string]any{"imports": imports}); err == nil {
					b.WriteString(`<script type="importmap">`)
					b.Write(m)
					b.WriteString(`</script>`)
				}
			}

			preloadPaths := make([]string, 0)
			modulePaths := make([]string, 0)
			globalPaths := make([]string, 0)
			for _, s := range e.ctx.Scripts.ScriptSeq {
				switch s.Type {
				case ScriptTypePreload:
					preloadPaths = append(preloadPaths, s.HashedPath)
				case ScriptTypeModule:
					modulePaths = append(modulePaths, s.HashedPath)
				case ScriptTypeGlobal:
					globalPaths = append(globalPaths, s.HashedPath)
				}
			}

			for _, path := range preloadPaths {
				fmt.Fprintf(b, `<link rel="modulepreload" href="%s">`, path)
			}
			for _, path := range modulePaths {
				fmt.Fprintf(b, `<script type="module" src="%s"></script>`, path)
			}
			for _, path := range globalPaths {
				fmt.Fprintf(b, `<script src="%s"></script>`, path)
			}

			return template.HTML(b.String()), nil
		},
		"getBank": func() []string {
			bank, err := mgr.i18nManager.GetBank(e.ctx.Lang.Code)
			if err != nil || bank == nil {
				bank = make([]string, 0)
			}
			return bank
		},
	}
	maps.Copy(funcs, mgr.funcMapBase())
	return funcs
}
