package i18n

import (
	"bufio"
	"io/fs"
	"log/slog"
	"path"
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
	Errors  map[string]string            `json:"errors"` // [key]
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
	Buckets         map[string]*LanguageBucket // [lang]bucket
	WordBanks       map[string][]string        // [lang]bank
	UsernameFormats map[string][]string        // [lang]formats
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
				if err != nil || d.IsDir() {
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

const (
	bucketSuffix = ".json"

	bankPrefix    = "bank"
	formatsPrefix = "formats"
)

func (mgr *I18nManager) processBucket(fileName string) (*LanguageBucket, error) {
	file, err := mgr.fs.Open(fileName)
	if err != nil {
		return nil, err
	}
	bucket := &LanguageBucket{}
	err = sonic.ConfigDefault.NewDecoder(file).Decode(&bucket)
	file.Close()
	if err != nil {
		return nil, err
	}
	return bucket, nil
}

func (mgr *I18nManager) Refresh() error {
	startTime := time.Now()

	rootFiles, err := fs.ReadDir(mgr.fs, ".")
	if err != nil {
		return err
	}

	newSnapshot := i18nSnapshot{
		Buckets:         make(map[string]*LanguageBucket),
		WordBanks:       make(map[string][]string),
		UsernameFormats: make(map[string][]string),
	}

	for _, rootEntry := range rootFiles {
		name := rootEntry.Name()

		if strings.HasSuffix(name, bucketSuffix) {
			var bucket *LanguageBucket
			if bucket, err = mgr.processBucket(name); err != nil || bucket == nil {
				slog.Error("Failed to process bucket in root", "name", name, "err", err, "bucket", bucket)
				continue
			}
			if bucket.Meta.Code == "" {
				bucket.Meta.Code = strings.TrimSuffix(name, bucketSuffix)
			}
			newSnapshot.Buckets[bucket.Meta.Code] = bucket
			continue
		}

		if rootEntry.IsDir() {
			subFiles, err := fs.ReadDir(mgr.fs, name)
			if err != nil {
				slog.Error("Failed to read subdir", "dir", name, "error", err)
				continue
			}

			for _, subFile := range subFiles {
				if subFile.IsDir() {
					continue
				}

				subFileName := subFile.Name()
				fullPath := path.Join(name, subFileName)

				if strings.HasSuffix(subFileName, bucketSuffix) {
					var bucket *LanguageBucket
					if bucket, err = mgr.processBucket(fullPath); err != nil || bucket == nil {
						slog.Error("Failed to process bucket in subdir", "name", subFileName, "path", fullPath, "err", err, "bucket", bucket)
						continue
					}
					if bucket.Meta.Code == "" {
						bucket.Meta.Code = strings.TrimSuffix(subFileName, bucketSuffix)
					}
					newSnapshot.Buckets[bucket.Meta.Code] = bucket
					slog.Info("Finished parsing a bucket", "path", fullPath)
				} else if strings.HasPrefix(subFileName, bankPrefix) {
					file, err := mgr.fs.Open(fullPath)
					if err != nil {
						slog.Error("Failed to open bank", "path", fullPath, "error", err)
						continue
					}

					var words []string
					scanner := bufio.NewScanner(file)
					for scanner.Scan() {
						if w := strings.TrimSpace(scanner.Text()); w != "" {
							words = append(words, w)
						}
					}
					file.Close()

					if len(words) != 2048 {
						slog.Warn("Bank word count mismatch", "lang", name, "count", len(words))
					}
					newSnapshot.WordBanks[name] = words
					slog.Info("Finished parsing a word bank", "path", fullPath)
				} else if strings.HasPrefix(subFileName, formatsPrefix) {
					file, err := mgr.fs.Open(fullPath)
					if err != nil {
						slog.Error("Failed to open formats", "path", fullPath, "error", err)
						continue
					}

					var words []string
					scanner := bufio.NewScanner(file)
					for scanner.Scan() {
						if w := strings.TrimSpace(scanner.Text()); w != "" {
							words = append(words, w)
						}
					}
					file.Close()

					if len(words) != 0 {
						newSnapshot.UsernameFormats[name] = words
					}
					slog.Info("Finished parsing username formats", "path", fullPath, "formats", len(words))
				}
			}
		}
	}

	mgr.snapshot.Store(&newSnapshot)

	select {
	case mgr.RefreshChan <- struct{}{}:
	default:
	}

	slog.Info("Refresh complete",
		"took", time.Since(startTime),
		"langs", len(newSnapshot.Buckets),
		"banks", len(newSnapshot.WordBanks),
	)
	return nil
}
