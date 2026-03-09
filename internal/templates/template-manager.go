package templates

import (
	"hash/crc32"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
)

const templateSuffix = ".html"

type TemplateKey struct {
	Kind string
	Name string
}

type TemplateWrapper struct {
	Template *template.Template
	Etag     string
}

func (t *TemplateWrapper) Execute(w io.Writer, data any) error {
	return t.Template.Execute(w, data)
}
func (t *TemplateWrapper) ExecuteTemplate(w io.Writer, name string, data any) error {
	return t.Template.ExecuteTemplate(w, name, data)
}

type templatesSnapshot struct {
	templates map[string]map[TemplateKey]*TemplateWrapper
}
type TemplateManager struct {
	RefreshChan    chan rune
	Templatefs     fs.FS
	Staticfs       fs.FS
	i18nManager    *i18n.I18nManager
	bufferPool     *BufferPool
	snapshot       atomic.Pointer[templatesSnapshot]
	assetHashes    atomic.Pointer[util.ReadOnlyMap[string, string]]
	staticRoot     string
	lastStaticScan time.Time
	lastHighestMod time.Time
}

func NewManager(templateFS, staticFS fs.FS, staticRoot string, i18nManager *i18n.I18nManager) (*TemplateManager, error) {
	mgr := &TemplateManager{
		Templatefs:  templateFS,
		Staticfs:    staticFS,
		i18nManager: i18nManager,
		RefreshChan: make(chan rune, 1),
		bufferPool:  NewBufferPool(65536),
		staticRoot:  staticRoot,
	}
	mgr.syncStaticAndCheck()
	if err := mgr.Refresh(); err != nil {
		return nil, err
	}
	mgr.lastHighestMod = time.Now()
	mgr.Watcher(time.Millisecond * 100)
	return mgr, nil
}

const (
	SignalReload = 'r'
	SignalCSS    = 'c'
)

var hashBufferPool = sync.Pool{
	New: func() any { b := make([]byte, 32*1024); return &b },
}

const maxSmallFile = 2 * 1024 * 1024

var crcTable = crc32.MakeTable(crc32.IEEE)

func generateETag(content []byte) string {
	checksum := crc32.Checksum(content, crcTable)
	return strconv.FormatUint(uint64(checksum), 16)
}

func calcHashPath(path string, size int64) (string, error) {
	var checksum uint32

	if size <= maxSmallFile {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		checksum = crc32.Checksum(data, crcTable)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()

		bufPtr := hashBufferPool.Get().(*[]byte)
		defer hashBufferPool.Put(bufPtr)

		h := crc32.New(crcTable)
		if _, err := io.CopyBuffer(h, f, *bufPtr); err != nil {
			return "", err
		}
		checksum = h.Sum32()
	}

	return strconv.FormatUint(uint64(checksum), 16), nil
}
func (mgr *TemplateManager) getAsset(path string) string {
	hashes := mgr.assetHashes.Load()
	if hashes == nil {
		return path
	}
	target, hash := getTargetPath(path, "/static/", "", *hashes)
	if target != "" {
		return target + "?v=" + hash
	}
	return path
}

func (mgr *TemplateManager) syncStaticAndCheck() string {
	ptr := mgr.assetHashes.Load()
	var currentHashes util.ReadOnlyMap[string, string]
	if ptr != nil {
		currentHashes = *ptr
	}

	changeFound := ""
	newHashes := make(map[string]string, currentHashes.Len())

	err := fs.WalkDir(mgr.Staticfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		urlPath := "/static/" + filepath.ToSlash(path)
		info, _ := d.Info()

		if info.ModTime().After(mgr.lastStaticScan) {
			fullPath := filepath.Join(mgr.staticRoot, path)
			newHash, _ := calcHashPath(fullPath, info.Size())
			newHashes[urlPath] = newHash

			if strings.HasSuffix(path, ".js") {
				changeFound = "hard"
			} else if changeFound != "hard" {
				changeFound = "soft"
			}
		} else {
			if h, ok := currentHashes.Lookup(urlPath); ok {
				newHashes[urlPath] = h
			} else {
				fullPath := filepath.Join(mgr.staticRoot, path)
				newHash, _ := calcHashPath(fullPath, info.Size())
				newHashes[urlPath] = newHash
				if changeFound == "" {
					changeFound = "soft"
				}
			}
		}
		return nil
	})

	if err != nil {
		slog.Error("Static walk error", "error", err)
		return ""
	}

	if changeFound != "" || len(newHashes) != currentHashes.Len() {
		mgr.lastStaticScan = time.Now()
		m := util.NewReadOnlyMap(newHashes)
		mgr.assetHashes.Store(&m)
	}

	return changeFound
}
func (mgr *TemplateManager) checkTemplates() bool {
	changed := false

	err := fs.WalkDir(mgr.Templatefs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, templateSuffix) {
			return nil
		}

		info, _ := d.Info()
		if info.ModTime().After(mgr.lastHighestMod) {
			mgr.lastHighestMod = info.ModTime()
			changed = true
		}
		return nil
	})

	if err != nil {
		slog.Error("Template walk error", "error", err)
	}

	return changed
}
func (mgr *TemplateManager) Watcher(delay time.Duration) {
	go func() {
		ticker := time.NewTicker(delay)
		defer ticker.Stop()

		var debounceTimer *time.Timer
		refreshTrigger := make(chan struct{}, 1)

		go func() {
			for range refreshTrigger {
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(time.Millisecond*500, func() {
					slog.Info("refresh signal recieved, refreshing...")
					err := mgr.Refresh()
					if err == nil {
						select {
						case mgr.RefreshChan <- SignalReload:
						default:
							slog.Debug("refreshChan full, skipping signal")
						}
					} else {
						slog.Error("error refreshing", "err", err)
					}
				})
			}
		}()
		for {
			select {
			case <-ticker.C:
				changeType := mgr.syncStaticAndCheck()
				if changeType != "" {
					sig := SignalCSS
					if changeType == "hard" {
						sig = SignalReload
					}

					select {
					case mgr.RefreshChan <- sig:
					default:
					}
				}

				if mgr.checkTemplates() {
					select {
					case refreshTrigger <- struct{}{}:
					default:
					}
				}
			case <-mgr.i18nManager.RefreshChan:
				select {
				case refreshTrigger <- struct{}{}:
				default:
				}
			}
		}
	}()
}
