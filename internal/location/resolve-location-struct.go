package location

import (
	"strings"
	"sync/atomic"

	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
)

type LocationResolveIn struct {
	exApi.FullAddress
	IP        string `json:"ip,omitempty"`
	cachedKey atomic.Pointer[string]
}

func (l *LocationResolveIn) Reset() {
	l.cachedKey.Store(nil)

	l.CityName = ""
	l.Country = ""
	l.State = ""
	l.IP = ""
	l.Lat = 0
	l.Lon = 0
}
func (lR *LocationResolveIn) Key(b *strings.Builder) string {
	if p := lR.cachedKey.Load(); p != nil {
		return *p
	}

	if lR.CityName != "" && lR.Country != "" {
		b.WriteString("a:")
		lR.LocationReadableAddress.WriteKey(b)
	}
	if lR.Lat != 0 || lR.Lon != 0 {
		b.WriteString("c:")
		lR.Coordinates.WriteKey(b)
	}
	if lR.IP != "" {
		b.WriteString("i:")
		b.WriteString(lR.IP)
	}

	finalStr := b.String()
	b.Reset()
	lR.cachedKey.Store(&finalStr)

	return finalStr
}

func (lR *LocationResolveIn) ResetKey() {
	lR.cachedKey.Store(nil)
}
