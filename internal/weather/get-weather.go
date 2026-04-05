package weather

import (
	"context"
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

func (wS *WeatherService) GetWeather(ctx context.Context, locationIn *location.LocationResolveIn, lang string, unit exApi.Unit, exclude exApi.Exclude) (*exApi.WeatherReport, []byte, error) {
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
	locationId := location.Id.Load()

	location.LocationReadableAddress.WriteKey(b)
	locKey := b.String()

	b.Reset()
	b.WriteString(locKey)
	b.WriteString(lang)

	selectedLangKey := b.String()

	if report, jsonBytes, ok := wS.cache.Get(selectedLangKey); ok {
		if report.Data.IsFresh() {
			if unit == exApi.UnitStandard && exclude == 0 {
				return report, jsonBytes, nil
			}
			return report.ConvertAndFilter(unit, exclude), nil, nil
		}
	}

	b.Reset()
	b.WriteString(locKey)
	b.WriteString("en")

	enKey := b.String()

	if lang != "en" {
		if enReport, _, ok := wS.cache.Get(enKey); ok {
			if enReport.Data.IsFresh() {
				localized, err := wS.localizeAndCache(selectedLangKey, enReport, lang)
				if err != nil {
					return nil, nil, err
				}
				return localized.ConvertAndFilter(unit, exclude), nil, err
			}
		}
	}

	type sfResult struct {
		report *exApi.WeatherReport
		raw    []byte
	}

	b.Reset()
	b.WriteString("f:")
	b.WriteString(locKey)

	weatherVal, err, _ := wS.sfG.Do(b.String(), func() (any, error) {
		var (
			wr  *exApi.WeatherReport
			raw []byte
			err error
		)
		if locationId == 0 {
			_, wr, raw, err = wS.FindWeatherCacheByAddress(ctx, &location.LocationReadableAddress, 10)
		} else {
			wr, raw, err = wS.FindWeatherCacheByLocId(ctx, locationId, 10)
		}
		if err == nil && wr != nil {
			return &sfResult{
				report: wr,
				raw:    raw,
			}, nil
		}

		apiData, err := wS.owClient.GetWeatherDataApi(ctx, location.Coordinates)
		if err != nil {
			return nil, err
		}

		final := apiData.ToReportGeoRes(location)
		wS.wg.Add(1)
		wS.saveQueue <- final

		return &sfResult{report: final.Report, raw: nil}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	result := weatherVal.(*sfResult)

	if result.raw != nil {
		wS.cache.AddHot(enKey, result.report, result.raw)
	} else {
		wS.cache.Add(enKey, result.report)
	}

	if lang == "en" && unit == exApi.UnitStandard && exclude == 0 {
		return result.report, result.raw, nil
	}

	localizedReport, err := wS.localizeAndCache(selectedLangKey, result.report, lang)
	if err != nil {
		return nil, nil, err
	}
	return localizedReport.ConvertAndFilter(unit, exclude), nil, err
}

func (wS *WeatherService) localizeAndCache(key string, raw *exApi.WeatherReport, lang string) (*exApi.WeatherReport, error) {
	trMap, err := wS.i18n.InternalWeatherMap(lang)
	if err != nil {
		return raw, nil
	}

	localizedReport := &exApi.WeatherReport{
		Data:    raw.Data.Localize(trMap),
		Address: raw.Address,
	}

	wS.cache.Add(key, localizedReport)

	return localizedReport, nil
}
