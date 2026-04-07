package i18n

import (
	"github.com/7apri/SimpleGOWebserver/pkg/util"
)

func (mgr *I18nManager) GetWeather(lang string, id int16) (string, error) {
	s := mgr.snapshot.Load()
	if s == nil {
		return "", ErrorNoSnapshot
	}

	bucket, ok := s.Buckets[lang]
	if !ok {
		bucket = s.Buckets["en"]
	}

	if bucket != nil {
		if val, ok := bucket.Weather[id]; ok {
			return val, nil
		}
	}

	return "", ErrorKeyNotFound
}
func (mgr *I18nManager) InternalWeatherMap(lang string) (util.ReadOnlyMap[int16, string], error) {
	s := mgr.snapshot.Load()
	if s == nil {
		return util.ReadOnlyMap[int16, string]{}, ErrorNoSnapshot
	}

	bucket, ok := s.Buckets[lang]
	if !ok {
		bucket = s.Buckets["en"]
	}

	if bucket != nil && bucket.Weather != nil {
		return util.NewReadOnlyMap(bucket.Weather), nil
	}

	return util.ReadOnlyMap[int16, string]{}, ErrorKeyNotFound
}
