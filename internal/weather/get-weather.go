package weather

import (
	"context"
	"strconv"
	"strings"
	"sync"

	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/7apri/SimpleGOWebserver/internal/location"
)

var builderPool = sync.Pool{
	New: func() any {
		b := new(strings.Builder)
		b.Grow(32)
		return b
	},
}

func (wS *WeatherService) GetWeather(ctx context.Context, locationIn *location.LocationResolveIn, lang string) (*exApi.WeatherReport, []byte, error) {
	b := builderPool.Get().(*strings.Builder)
	defer func() {
		b.Reset()
		builderPool.Put(b)
	}()

	val, err, _ := wS.sfG.Do(locationIn.Key(b), func() (any, error) {
		location, _, err := wS.locationService.ResolveLocation(ctx, locationIn)
		if err != nil {
			return nil, err
		}
		return location, nil
	})
	if err != nil {
		return nil, nil, err
	}

	location := val.(*exApi.GeoResult)
	locationId := location.GetId()

	var numBuf [20]byte

	b.WriteString("i:")
	b.Write(strconv.AppendInt(numBuf[:0], locationId, 10))
	b.WriteRune(':')
	idString := b.String()

	b.Reset()
	b.WriteString(idString)
	b.WriteString(lang)

	selectedLangKey := b.String()

	if report, jsonBytes, ok := wS.cache.Get(selectedLangKey); ok {
		if report.Data.IsFresh() {
			return report, jsonBytes, nil
		}
	}

	b.Reset()
	b.WriteString(idString)
	b.WriteString("en")

	enKey := b.String()

	if lang != "en" {
		if enReport, _, ok := wS.cache.Get(enKey); ok {
			if enReport.Data.IsFresh() {
				return wS.localizeAndCache(selectedLangKey, enReport, lang)
			}
		}
	}

	type sfResult struct {
		report *exApi.WeatherReport
		raw    []byte
	}

	b.Reset()
	b.WriteString("f:")
	b.WriteString(idString)

	weatherVal, err, _ := wS.sfG.Do(b.String(), func() (any, error) {
		wd, raw, err := wS.FindWeatherCacheByLocId(ctx, locationId, 10)
		if err == nil && wd != nil {
			return &sfResult{
				report: wd,
				raw:    raw,
			}, nil
		}

		apiData, err := wS.owClient.GetWeatherDataApi(ctx, location.Coordinates)
		if err != nil {
			return nil, err
		}

		final := apiData.ToReportId(locationId, &location.LocationReadableLocalizedAddress)
		wS.wg.Add(1)
		wS.saveQueue <- final

		return &sfResult{report: final.Report, raw: nil}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	result := weatherVal.(*sfResult)

	wS.cache.Add(enKey, result.report)

	if lang == "en" {
		return result.report, result.raw, nil
	}

	return wS.localizeAndCache(selectedLangKey, result.report, lang)
}

func (wS *WeatherService) localizeAndCache(key string, raw *exApi.WeatherReport, lang string) (*exApi.WeatherReport, []byte, error) {
	if report, bytes, ok := wS.cache.Get(key); ok {
		return report, bytes, nil
	}

	trMap := wS.i18n[lang]

	localizedReport := &exApi.WeatherReport{
		Data:    raw.Data.Localize(trMap),
		Address: raw.Address,
	}

	wS.cache.Add(key, localizedReport)

	return localizedReport, nil, nil
}
