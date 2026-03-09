package templates

import (
	"bytes"
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
}
func (c *BakeContext) Reset() {
	clear(c.Meta)
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
		AllLangs: c.AllLangs,
		Depth:    c.Depth + 1,
		Scripts:  c.Scripts,
		Meta:     make(metadata),
	}
}
func (c *BakeContext) SubContextInto(dest *BakeContext) {
	dest.Lang = c.Lang
	dest.Name = ""
	dest.Layout = ""
	dest.AllLangs = c.AllLangs
	dest.Body = c.Body
	dest.Depth = c.Depth + 1
	dest.Scripts = c.Scripts
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
	b := mgr.bufferPool.Get()
	defer mgr.bufferPool.Put(b)
	b.Reset()

	err := mgr.executeTemplate(c.Content, e, b)
	if err != nil {
		return "", err
	}
	p := b.String()
	b.Reset()

	if e.ctx.Layout != "" {
		targetPath, l := e.getTargetPath(e.ctx.Layout, "layouts/")

		if targetPath != "" {
			e.ctx.Body = htmlTmpl.HTML(p)
			err := mgr.executeTemplate(l.Content, e, b)
			if err != nil {
				return "", err
			}
			return b.String(), nil
		}
	}

	return p, nil
}
