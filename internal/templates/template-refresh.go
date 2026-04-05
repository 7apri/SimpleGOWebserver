package templates

import (
	"cmp"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"runtime"
	"slices"
	"strings"
	"sync"
	textTmpl "text/template"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"golang.org/x/sync/errgroup"
)

type RawTemplate struct {
	Content *textTmpl.Template
	Meta    metadata
}

func (mgr *TemplateManager) Refresh() error {
	startTime := time.Now()

	templates := make(map[string]*RawTemplate)

	err := fs.WalkDir(mgr.Templatefs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, templateSuffix) {
			return nil
		}

		b, err := fs.ReadFile(mgr.Templatefs, path)
		if err != nil {
			return err
		}
		raw := string(b)

		meta, cleanContent := getMetadata(raw)
		t, err := textTmpl.New(path).
			Delims("[[", "]]").
			Funcs(mgr.funcMapBake(nil)).
			Parse(cleanContent)

		if err != nil {
			return fmt.Errorf("error parsing %s: %w", path, err)
		}

		meta["path"] = path

		templates[path] = &RawTemplate{
			Content: t,
			Meta:    meta,
		}
		return nil
	})
	if err != nil {
		return err
	}

	newSnapshot := templatesSnapshot{
		make(map[string]map[TemplateKey]*TemplateWrapper),
	}
	langs := mgr.i18nManager.GetAvailableLangsWhole()
	slices.SortFunc(langs, func(a, b i18n.Lang) int {
		return cmp.Compare(a.Code, b.Code)
	})

	var mu sync.Mutex
	g, groupCtx := errgroup.WithContext(context.Background())
	g.SetLimit(runtime.GOMAXPROCS(0))

	for _, lang := range langs {
		slog.Info("proccessing templates", "lang", lang.Code)

		newSnapshot.templates[lang.Code] = make(map[TemplateKey]*TemplateWrapper)

		for path, file := range templates {
			if strings.HasPrefix(path, "layouts/") || strings.HasPrefix(path, "components/") {
				continue
			}

			path, file := path, file
			lang := lang

			g.Go(func() error {
				if groupCtx.Err() != nil {
					return nil
				}

				ctx := bakeContextPool.Get().(*BakeContext)
				ctx.Reset()
				defer bakeContextPool.Put(ctx)

				ctx.InsertMeta(file.Meta)

				if ctx.Scripts == nil {
					ctx.Scripts = make(map[string]struct{})
				}

				ctx.Lang = lang
				ctx.AllLangs = langs

				bakedPage, err := mgr.bake(file, &bakeEnv{
					files: util.NewReadOnlyMap(templates),
					ctx:   ctx,
				})
				if err != nil {
					return fmt.Errorf("[Page: %s] [Lang: %s] bake failed: %w", path, lang.Code, err)
				}
				bakedPage = strings.TrimSpace(bakedPage)
				tag := generateETag([]byte(bakedPage))

				t := template.New(path).Funcs(mgr.funcMapExec(lang))

				t, err = t.Parse(bakedPage)
				if err != nil {
					return fmt.Errorf("[%s][%s] parse failed: %w", lang.Code, path, err)
				}

				kind := ctx.Layout
				if k := ctx.Meta["kind"]; k != "" {
					kind = k
				}

				mu.Lock()
				newSnapshot.templates[lang.Code][TemplateKey{Kind: kind, Name: ctx.Name}] = &TemplateWrapper{
					Template: t,
					Etag:     tag,
				}
				mu.Unlock()
				return nil
			})

		}
	}
	if err := g.Wait(); err != nil {
		return err
	}
	mgr.snapshot.Store(&newSnapshot)

	total := 0
	for _, l := range newSnapshot.templates {
		total += len(l)
	}
	englishKeys := make([]string, 0, total/max(len(langs), 1))

	for k := range newSnapshot.templates["en"] {
		englishKeys = append(englishKeys, fmt.Sprintf("%s@%s", k.Name, k.Kind))
	}

	slog.Info("Refresh complete: templates",
		slog.Group("metrics",
			slog.Int("langs", len(langs)),
			slog.Int("total", total),
			slog.String("keys", strings.Join(englishKeys, ",")),
			slog.Duration("took", time.Since(startTime)),
		),
	)

	return nil
}
