package location

import (
	"context"
	"fmt"
	"strings"
	"sync"

	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
)

var builderPool = sync.Pool{
	New: func() any {
		b := new(strings.Builder)
		b.Grow(32)
		return b
	},
}

func (lS *LocationService) ResolveLocation(ctx context.Context, locationIn *LocationResolveIn) (*exApi.GeoResult, []byte, error) {
	b := builderPool.Get().(*strings.Builder)
	defer func() {
		builderPool.Put(b)
	}()

	if data, jsonBytes, ok := lS.cache.Get(locationIn.Key(b)); ok {
		return data, jsonBytes, nil
	}

	if locationIn.IP != "" && locationIn.CityName == "" {
		val, err, _ := lS.sfG.Do(locationIn.IP, func() (any, error) {
			return lS.ipClient.IpToCoordinates(ctx, locationIn.IP)
		})
		if err == nil {
			ipRes := val.(*exApi.IpGeoResult)
			locationIn.LocationReadableAddress = ipRes.GetAddress()
			locationIn.Coordinates = ipRes.Coordinates

			locationIn.ResetKey()

			if data, jsonBytes, ok := lS.cache.Get(locationIn.Key(b)); ok {
				return data, jsonBytes, nil
			}
		}
	}

	val, err, _ := lS.sfG.Do(locationIn.Key(b), func() (any, error) {
		var result *exApi.GeoResult
		var err error

		if locationIn.CityName != "" {
			result, err = lS.FindExactLocationByAddress(ctx, &locationIn.LocationReadableAddress)
			if err != nil {
				data, apiErr := lS.owClient.Geolocate(ctx, &locationIn.LocationReadableAddress)
				if apiErr == nil && len(data) > 0 {
					result = &data[0]
					lS.wg.Add(1)
					lS.saveQueue <- result
				}
			}
		} else if locationIn.Lat != 0 {
			result, err = lS.FindLocationByCoords(ctx, &locationIn.Coordinates)
			if err != nil {
				data, apiErr := lS.owClient.ReverseGeolocate(ctx, &locationIn.Coordinates)
				if apiErr == nil && len(data) > 0 {
					result = &data[0]
					lS.wg.Add(1)
					lS.saveQueue <- result
				}
			}
		}

		if result == nil {
			return nil, fmt.Errorf("location not found")
		}
		return result, nil
	})

	if err != nil {
		return nil, nil, err
	}

	finalResult := val.(*exApi.GeoResult)

	b.WriteString("a:")
	finalResult.LocationReadableAddress.WriteKey(b)
	lS.cache.Add(b.String(), finalResult)
	b.Reset()

	b.WriteString("c:")
	finalResult.Coordinates.WriteKey(b)
	lS.cache.Add(b.String(), finalResult)
	b.Reset()

	lS.cache.Add(locationIn.Key(b), finalResult)

	return finalResult, nil, nil
}
