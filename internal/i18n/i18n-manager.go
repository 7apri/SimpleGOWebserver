package i18n

import (
	"io/fs"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
)

type Lang struct {
	Code           string `json:"code"`
	SwitchSentence string `json:"switch"`
}
type LanguageBucket struct {
	Static  map[string]string            `json:"static"` // [key]
	Client  map[string]map[string]string `json:"client"` // [script][key]
	Weather map[int16]string             `json:"-"`      // [id]
	Meta    Lang                         `json:"meta"`   // Lang code and SwitchSentence
}

func (b *LanguageBucket) UnmarshalJSON(data []byte) error {
	type Alias LanguageBucket
	aux := &struct {
		RawWeather map[string]string `json:"weather"`
		*Alias
	}{
		Alias: (*Alias)(b),
	}

	if err := sonic.Unmarshal(data, &aux); err != nil {
		return err
	}

	idMap := getWeatherIdToKey()
	b.Weather = make(map[int16]string, len(idMap))

	for id, key := range idMap {
		if val, ok := aux.RawWeather[key]; ok {
			b.Weather[id] = val
		}
	}

	return nil
}

type i18nSnapshot struct {
	Buckets map[string]*LanguageBucket // [lang][page]
}
type I18nManager struct {
	RefreshChan chan struct{}
	fs          fs.FS
	snapshot    atomic.Pointer[i18nSnapshot]
}

func NewManager(i18nFS fs.FS) (*I18nManager, error) {
	mgr := &I18nManager{
		fs:          i18nFS,
		RefreshChan: make(chan struct{}, 1),
	}
	if err := mgr.Refresh(); err != nil {
		return nil, err
	}
	<-mgr.RefreshChan
	mgr.Watcher(time.Second)
	return mgr, nil
}

func (mgr *I18nManager) Watcher(delay time.Duration) {
	go func() {
		var lastHighestMod time.Time
		for {
			time.Sleep(delay)

			currentHighest := lastHighestMod

			err := fs.WalkDir(mgr.fs, ".", func(path string, d fs.DirEntry, err error) error {
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
func (mgr *I18nManager) Refresh() error {
	startTime := time.Now()

	files, err := fs.ReadDir(mgr.fs, ".")
	if err != nil {
		return err
	}
	newSnapshot := i18nSnapshot{
		Buckets: make(map[string]*LanguageBucket, len(files)),
	}

	for i, f := range files {
		fileName := f.Name()
		if f.IsDir() || !strings.HasSuffix(fileName, ".json") {
			continue
		}
		i := i + 1
		slog.Info("Processing translation", "name", fileName, "number", i, "left", len(files)-i)

		file, err := mgr.fs.Open(fileName)
		if err != nil {
			slog.Error("Failed to open", "file", fileName, "error", err)
			continue
		}

		bucket := &LanguageBucket{}
		err = sonic.ConfigDefault.NewDecoder(file).Decode(&bucket)
		file.Close()
		if err != nil {
			slog.Error("Failed to decode", "error", err)
			continue
		}
		if bucket.Meta.Code == "" {
			bucket.Meta.Code = strings.TrimSuffix(fileName, ".json")
		}

		newSnapshot.Buckets[bucket.Meta.Code] = bucket
	}

	mgr.snapshot.Store(&newSnapshot)
	mgr.RefreshChan <- struct{}{}

	langs := make([]string, 0, len(newSnapshot.Buckets))
	for lang := range newSnapshot.Buckets {
		langs = append(langs, lang)
	}

	slog.Info("Refresh complete: i18n ",
		slog.Group("metrics",
			slog.String("langs", strings.Join(langs, ",")),
			slog.Int("total", len(newSnapshot.Buckets)),
			slog.Duration("took", time.Since(startTime)),
		))
	return nil
}
