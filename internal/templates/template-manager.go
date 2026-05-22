package templates

import (
	"hash/crc32"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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

type AssetInfo struct {
	Hash string
	Deps []string
}

type TemplateManager struct {
	RefreshChan             chan rune
	Templatefs              fs.FS
	Staticfs                fs.FS
	i18nManager             *i18n.I18nManager
	bufferPool              *util.BufferPool
	snapshot                atomic.Pointer[templatesSnapshot]
	assetInfo               atomic.Pointer[util.ReadOnlyMap[string, AssetInfo]]
	staticRoot              string
	lastHighestModAssets    time.Time
	lastHighestModTemplates time.Time
}

func NewManager(templateFS, staticFS fs.FS, staticRoot string, i18nManager *i18n.I18nManager) (*TemplateManager, error) {
	mgr := &TemplateManager{
		Templatefs:  templateFS,
		Staticfs:    staticFS,
		i18nManager: i18nManager,
		RefreshChan: make(chan rune, 1),
		bufferPool:  util.NewBufferPool(65536),
		staticRoot:  staticRoot,
	}
	mgr.syncStaticAndCheck()

	if err := mgr.Refresh(); err != nil {
		slog.Error("initial template refresh failed", "err", err)
	}
	mgr.lastHighestModTemplates = time.Now()

	mgr.Watcher(time.Millisecond * 300)

	return mgr, nil
}

const (
	SignalReload = 'r'
	SignalCSS    = 'c'
	maxSmallFile = 2 * 1024 * 1024
)

var hashBufferPool = sync.Pool{
	New: func() any { b := make([]byte, 32*1024); return &b },
}

var crcTable = crc32.MakeTable(crc32.IEEE)

func generateETag(content []byte) string {
	checksum := crc32.Checksum(content, crcTable)
	return strconv.FormatUint(uint64(checksum), 16)
}

func calcHashPath(path string, size int64) (string, error) {
	var hash string

	if size <= maxSmallFile {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		hash = calcHashBytes(data)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		hash, err = calcHashFile(f)
		if err != nil {
			return "", err
		}
	}

	return hash, nil
}
func calcHashBytes(content []byte) string {
	return strconv.FormatUint(uint64(crc32.Checksum(content, crcTable)), 16)
}
func calcHashFile(f *os.File) (string, error) {
	bufPtr := hashBufferPool.Get().(*[]byte)
	defer hashBufferPool.Put(bufPtr)

	h := crc32.New(crcTable)
	if _, err := io.CopyBuffer(h, f, *bufPtr); err != nil {
		return "", err
	}
	return strconv.FormatUint(uint64(h.Sum32()), 16), nil
}

var importRegex = regexp.MustCompile(`(?m)\b(?:import|export)\b(?:[^'"]*from)?\s*['"](\.\.?\/[^'"]+)['"]`)

func getDeps(content []byte) []string {
	matches := importRegex.FindAllSubmatch(content, -1)
	var deps []string
	for _, m := range matches {
		deps = append(deps, string(m[1]))
	}
	return deps
}

func (mgr *TemplateManager) getAsset(path string) string {
	assetInfo := mgr.assetInfo.Load()
	if assetInfo == nil {
		return path
	}
	target, info := getTargetPath(path, "/static/", "", *assetInfo)
	if target != "" {
		return target + "?v=" + info.Hash
	}
	return path
}

func (mgr *TemplateManager) processFile(path string, fileSize int64) (AssetInfo, bool, error) {
	var info AssetInfo
	var err error
	isJs := strings.HasSuffix(path, ".js")

	if isJs {
		content, err := fs.ReadFile(mgr.Staticfs, path)
		if err != nil {
			return AssetInfo{}, isJs, err
		}
		info.Hash = calcHashBytes(content)
		rawDeps := getDeps(content)

		baseDir := filepath.Dir(path)
		for _, d := range rawDeps {
			resolved := filepath.Join(baseDir, d)

			relToJs, err := filepath.Rel("js", resolved)
			if err == nil {
				info.Deps = append(info.Deps, filepath.ToSlash(relToJs))
			}
		}
	} else {
		fullPath := filepath.Join(mgr.staticRoot, path)
		info.Hash, err = calcHashPath(fullPath, fileSize)
		if err != nil {
			return AssetInfo{}, isJs, err
		}
	}
	return info, isJs, nil
}

func (mgr *TemplateManager) syncStaticAndCheck() string {
	ptr := mgr.assetInfo.Load()
	var currentInfo util.ReadOnlyMap[string, AssetInfo]
	if ptr != nil {
		currentInfo = *ptr
	}

	changeFound := ""
	highestMod := mgr.lastHighestModAssets
	newInfo := make(map[string]AssetInfo, currentInfo.Len())

	err := fs.WalkDir(mgr.Staticfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		urlPath := "/static/" + filepath.ToSlash(path)
		info, _ := d.Info()
		modTime := info.ModTime()

		if modTime.After(highestMod) {
			highestMod = modTime
		}

		if modTime.After(mgr.lastHighestModAssets) {
			new, isJs, err := mgr.processFile(path, info.Size())
			if err != nil {
				return err
			}

			newInfo[urlPath] = new

			if isJs {
				changeFound = "hard"
			} else if changeFound != "hard" {

				changeFound = "soft"
			}
		} else {
			if h, ok := currentInfo.Lookup(urlPath); ok {
				newInfo[urlPath] = h
			} else {
				new, _, err := mgr.processFile(path, info.Size())
				if err != nil {
					return err
				}
				newInfo[urlPath] = new
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

	if changeFound != "" || len(newInfo) != currentInfo.Len() {
		mgr.lastHighestModAssets = highestMod
		m := util.NewReadOnlyMap(newInfo)
		mgr.assetInfo.Store(&m)
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
		if info.ModTime().After(mgr.lastHighestModTemplates) {
			mgr.lastHighestModTemplates = info.ModTime()
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

		refreshTrigger := make(chan struct{}, 1)

		go func() {
			var debounce <-chan time.Time

			for {
				select {
				case <-refreshTrigger:
					debounce = time.After(600 * time.Millisecond)

				case <-debounce:
					debounce = nil

					slog.Info("refresh signal received, refreshing...")
					if err := mgr.Refresh(); err == nil {
						select {
						case mgr.RefreshChan <- SignalReload:
						default:
						}
					} else {
						slog.Error("error refreshing", "err", err)
					}
				}
			}
		}()
		for {
			select {
			case <-ticker.C:
				refresh := mgr.checkTemplates()

				changeType := mgr.syncStaticAndCheck()
				if changeType != "" {
					if changeType != "hard" {
						sig := SignalCSS

						select {
						case mgr.RefreshChan <- sig:
						default:
						}
					} else {
						refresh = true
					}
				}

				if refresh {
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
