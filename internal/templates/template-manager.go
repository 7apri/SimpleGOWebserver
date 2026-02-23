package templates

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
)

const templateSuffix = ".html"

type templateKey struct {
	kind string
	name string
}

type templatesSnapshot struct {
	templates    map[string]map[templateKey]*TemplateWrapper
	langElements string
}
type Manager struct {
	fs          fs.FS
	i18nManager *i18n.Manager
	snapshot    atomic.Pointer[templatesSnapshot]
}

func NewManager(templateFS fs.FS, i18nManager *i18n.Manager) (*Manager, error) {
	mgr := &Manager{
		fs:          templateFS,
		i18nManager: i18nManager,
	}
	if err := mgr.Refresh(); err != nil {
		return nil, err
	}
	mgr.Watcher(time.Second)
	return mgr, nil
}
func (mgr *Manager) Watcher(delay time.Duration) {
	go func() {
		ticker := time.NewTicker(delay)
		defer ticker.Stop()

		var lastHighestMod time.Time

		fs.WalkDir(mgr.fs, ".", func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, templateSuffix) {
				if info, err := d.Info(); err == nil {
					if info.ModTime().After(lastHighestMod) {
						lastHighestMod = info.ModTime()
					}
				}
			}
			return nil
		})

		for {
			select {
			case <-ticker.C:
				currentHighest := lastHighestMod
				hasChanged := false

				fs.WalkDir(mgr.fs, ".", func(path string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() || !strings.HasSuffix(path, templateSuffix) {
						return nil
					}

					info, _ := d.Info()
					if info.ModTime().After(lastHighestMod) {
						currentHighest = info.ModTime()
						hasChanged = true
					}
					return nil
				})

				if hasChanged {
					slog.Info("Template file change detected", "at", currentHighest)
					mgr.Refresh()
					lastHighestMod = currentHighest
				}

			case <-mgr.i18nManager.RefreshChan:
				slog.Info("i18n refresh signal received, re-baking templates")
				mgr.Refresh()
			}
		}
	}()
}
func (mgr *Manager) Refresh() error {
	langs := mgr.i18nManager.GetAvailableLangsWhole()
	b := new(strings.Builder)
	b.Grow(len(langs) * 32)
	for _, lang := range langs {
		fmt.Fprintf(b,
			`<li class="lang" data-lang="%s">
				<button class="grd pli-c fil-width py-xs" aria-label="%s">
					<svg class="icon color" aria-hidden="true">
						<use href="/static/assets/iconBundle.svg#%s"></use>
					</svg>
				</button>
			 </li>`,
			lang.Code, lang.SwitchSentence, lang.Code,
		)
	}

	newSnapshot := templatesSnapshot{
		make(map[string]map[templateKey]*TemplateWrapper),
		b.String(),
	}
	rawFiles := make(map[string]string)

	err := fs.WalkDir(mgr.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, templateSuffix) {
			return nil
		}
		b, _ := fs.ReadFile(mgr.fs, path)
		rawFiles[path] = string(b)
		return nil
	})
	if err != nil {
		return err
	}
	for _, lang := range langs {
		slog.Info("proccessing templates", "lang", lang.Code)
		newSnapshot.templates[lang.Code] = make(map[templateKey]*TemplateWrapper)

		for path, content := range rawFiles {
			if strings.HasPrefix(path, "layouts/") {
				continue
			}
			parts := strings.Split(path, "/")
			if len(parts) < 2 {
				continue
			}

			folder := strings.TrimSuffix(parts[0], "s")
			layoutKey := fmt.Sprintf("layouts/%s.html", folder)
			layoutHTML, hasLayout := rawFiles[layoutKey]

			cleanName := strings.TrimSuffix(strings.Join(parts[1:], "/"), templateSuffix)
			fullName := fmt.Sprintf("%s:%s", folder, cleanName)

			baked := mgr.bake(content, cleanName, lang, newSnapshot)

			var (
				t          *template.Template
				entryPoint string
				err        error
			)

			if hasLayout {
				bakedLayout := mgr.bake(layoutHTML, cleanName, lang, newSnapshot)
				entryPoint = getEntryPoint(bakedLayout)

				t = template.New(fullName).Funcs(mgr.funcMap(lang))
				t, err = t.Parse(bakedLayout)
				if err == nil {
					_, err = t.Parse(baked)
				}
			} else {
				t, err = template.New(fullName).Funcs(mgr.funcMap(lang)).Parse(baked)
			}

			if err != nil {
				slog.Error("Template error", "name", fullName, "path", path, "err", err)
				continue
			}

			key := templateKey{kind: folder, name: cleanName}

			newSnapshot.templates[lang.Code][key] = &TemplateWrapper{
				t: t,
				e: entryPoint,
			}
		}
	}

	totalBaked := 0
	for _, l := range newSnapshot.templates {
		totalBaked += len(l)
	}
	var englishKeys strings.Builder
	for k := range newSnapshot.templates["en"] {
		fmt.Fprintf(&englishKeys, "|%s/%s|", k.name, k.kind)
	}

	mgr.snapshot.Store(&newSnapshot)
	slog.Info("template_refresh_complete",
		slog.Group("metrics",
			slog.Int("langs", len(langs)),
			slog.Int("total_tmpl", totalBaked),
			slog.String("english_keys", englishKeys.String()),
		),
	)
	return nil
}

var defineRegex = regexp.MustCompile(`{{\s*define\s*\"([^\"]+)\"\s*}}`)

func getEntryPoint(layoutHTML string) string {
	match := defineRegex.FindStringSubmatch(layoutHTML)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

var trRegex = regexp.MustCompile(`{{\s*tr\s+"([^"]+)"\s*}}`)

var dynamicStringRegex = regexp.MustCompile(`({{\s*.*?)(getDynamicString)(.*?\s*}})`)
var dynamicJSONRegex = regexp.MustCompile(`{{\s*.*?(getDynamicJSON).*?\s*}}`)
var langElemetsRegex = regexp.MustCompile(`{{\s*.*?(getLangElements).*?\s*}}`)

var fullTitleRegex = regexp.MustCompile(`{{\s*fullTitle\s*}}`)
var langRegex = regexp.MustCompile(`{{\s*getLang\s*}}`)

func (tm *Manager) bake(content, pageName string, lang i18n.Lang, currentSnapshot templatesSnapshot) string {
	content = trRegex.ReplaceAllStringFunc(content, func(match string) string {
		key := trRegex.FindStringSubmatch(match)[1]
		val := tm.i18nManager.GetPageStatic(lang.Code, key)
		return val
	})

	content = langRegex.ReplaceAllString(content, lang.Code)
	content = fullTitleRegex.ReplaceAllStringFunc(content, func(match string) string {
		title := tm.i18nManager.GetPageStatic(lang.Code, fmt.Sprintf("title_%s", pageName))
		return title
	})

	content = dynamicStringRegex.ReplaceAllStringFunc(content, func(match string) string {
		jsCode := tm.i18nManager.GetPageDynamic(lang.Code, pageName)
		replacement := strconv.Quote(string(jsCode))

		subs := dynamicStringRegex.FindStringSubmatch(match)
		return subs[1] + replacement + subs[3]
	})
	content = dynamicJSONRegex.ReplaceAllStringFunc(content, func(match string) string {
		jsCode := tm.i18nManager.GetPageDynamic(lang.Code, pageName)
		return string(jsCode)
	})
	content = langElemetsRegex.ReplaceAllStringFunc(content, func(match string) string {
		return currentSnapshot.langElements
	})

	return content
}
func (tm *Manager) funcMap(lang i18n.Lang) template.FuncMap {
	return template.FuncMap{}
}
