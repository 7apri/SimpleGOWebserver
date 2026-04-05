package templates

import (
	"bytes"
	"errors"
	"fmt"
	htmlTmpl "html/template"
	"strings"
	"sync"
	textTmpl "text/template"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
)

type BufferPool struct {
	pool    sync.Pool
	maxSize int
}

func NewBufferPool(maxSize int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return new(bytes.Buffer)
			},
		},
		maxSize: maxSize,
	}
}

func (p *BufferPool) Get() *bytes.Buffer {
	b := p.pool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

func (p *BufferPool) Put(b *bytes.Buffer) {
	if b.Cap() <= p.maxSize {
		p.pool.Put(b)
	}
}

var bakeContextPool = sync.Pool{
	New: func() any {
		return &BakeContext{}
	},
}

type BakeContext struct {
	Lang     i18n.Lang
	AllLangs []i18n.Lang
	Meta     metadata
	Name     string
	Layout   string
	Scripts  map[string]struct{}
	Defines  map[string]string
	Body     htmlTmpl.HTML
	Depth    uint8
	Data     any
}

func (c *BakeContext) InsertMeta(incoming metadata) {
	if c.Meta == nil {
		c.Meta = make(metadata)
	}
	for k, v := range incoming {
		switch k {
		case "layout":
			c.Layout = v
		case "name":
			c.Name = v
		default:
			c.Meta[k] = v
		}
	}
	if c.Name == "" {
		if path, ok := c.Meta["path"]; ok {
			c.Name = strings.TrimSuffix(path, templateSuffix)
		}
	}
}
func (c *BakeContext) Reset() {
	if c.Meta == nil {
		c.Meta = make(metadata)
	} else {
		clear(c.Meta)
	}
	if c.Defines == nil {
		c.Defines = make(map[string]string)
	} else {
		clear(c.Defines)
	}
	c.Scripts = nil
	c.Lang = i18n.Lang{}
	c.AllLangs = nil
	c.Data = nil
	c.Name = ""
	c.Layout = ""
	c.Depth = 0
}
func (c *BakeContext) NewSubContext() *BakeContext {
	return &BakeContext{
		Lang:     c.Lang,
		Name:     c.Name,
		AllLangs: c.AllLangs,
		Depth:    c.Depth + 1,
		Scripts:  c.Scripts,
		Meta:     make(metadata),
	}
}
func (c *BakeContext) SubContextInto(dest *BakeContext) {
	dest.Lang = c.Lang
	dest.Name = c.Name
	dest.Layout = ""
	dest.AllLangs = c.AllLangs
	dest.Body = c.Body
	dest.Depth = c.Depth + 1
	dest.Scripts = c.Scripts
	if dest.Defines == nil {
		dest.Defines = make(map[string]string)
	} else {
		dest.Defines = c.Defines
	}
	if dest.Meta == nil {
		dest.Meta = make(metadata)
	} else {
		clear(dest.Meta)
	}
}

type bakeEnv struct {
	ctx   *BakeContext
	files util.ReadOnlyMap[string, *RawTemplate]
}

func (e bakeEnv) withCtx(ctx *BakeContext) *bakeEnv {
	e.ctx = ctx
	return &e
}
func (e *bakeEnv) isCtxNil() bool {
	if e == nil || e.ctx == nil {
		return true
	}
	return false
}

func (e *bakeEnv) getTargetPath(b, f string) (string, *RawTemplate) {
	return getTargetPath(b, f, templateSuffix, e.files)
}
func getTargetPath[V any](base, folder, suffix string, files util.ReadOnlyMap[string, V]) (string, V) {
	if suffix != "" && !strings.HasSuffix(base, suffix) {
		base += suffix
	}

	if v, exists := files.Lookup(base); exists {
		return base, v
	}

	if folder != "" {
		if !strings.HasPrefix(base, folder) {
			if !strings.HasSuffix(folder, "/") {
				folder += "/"
			}
			fullPath := folder + base
			if v, exists := files.Lookup(fullPath); exists {
				return fullPath, v
			}
		}
	}

	var zero V
	return "", zero
}

func (mgr *TemplateManager) importTemplate(template *RawTemplate, e *bakeEnv, data any) (string, error) {
	childCtx := bakeContextPool.Get().(*BakeContext)
	defer bakeContextPool.Put(childCtx)
	childCtx.Reset()

	e.ctx.SubContextInto(childCtx)
	childCtx.InsertMeta(template.Meta)

	if data != nil {
		childCtx.Data = data
	} else {
		childCtx.Data = make(map[string]any)
	}

	return mgr.bake(template, e.withCtx(childCtx))
}

func (mgr *TemplateManager) executeTemplate(t *textTmpl.Template, e *bakeEnv, b *bytes.Buffer) error {
	tmpl, _ := t.Clone()
	tmpl.Funcs(mgr.funcMapBake(e))

	if err := tmpl.Execute(b, e.ctx); err != nil {
		return err
	}
	return nil
}

func (mgr *TemplateManager) bake(
	c *RawTemplate,
	e *bakeEnv,
) (string, error) {
	if e == nil {
		return "", fmt.Errorf("bake env cant be nil")
	}
	if c == nil {
		return "", errors.New("template cant be nil")
	}

	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)

	err := mgr.executeTemplate(c.Content, e, b)
	if err != nil {
		return "", err
	}
	p := b.String()
	b.Reset()

	if e.ctx.Layout != "" {

		targetPath, l := e.getTargetPath(e.ctx.Layout, "layouts/")
		if e.ctx.Meta["path"] == targetPath {
			return "", fmt.Errorf("cant import itself: %s", targetPath)
		}
		if e.ctx.Depth > 10 {
			return "", fmt.Errorf("max import depth exceeded at %s", targetPath)
		}

		if targetPath != "" {
			e.ctx.Body = htmlTmpl.HTML(p)
			return mgr.importTemplate(l, e, e.ctx.Data)
		} else {
			return "", fmt.Errorf("layout %s not found", e.ctx.Layout)
		}

	}

	return p, nil
}
