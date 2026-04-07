package exApi

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
)

type GeoResult struct {
	Id atomic.Int64
	LocationReadableLocalizedAddress
	Coordinates
}

func (r *GeoResult) GetId() int64 {
	if id := r.Id.Load(); id != 0 {
		return id
	}

	for range 200 {
		if id := r.Id.Load(); id != 0 {
			return id
		}
		time.Sleep(time.Millisecond)
	}

	return r.Id.Load()
}
func (r *GeoResult) Marshal() ([]byte, error) {
	return sonic.Marshal(&struct {
		Id int64 `json:"id,omitempty"`
		*LocationReadableLocalizedAddress
		*Coordinates
	}{
		Id:                               r.Id.Load(),
		LocationReadableLocalizedAddress: &r.LocationReadableLocalizedAddress,
		Coordinates:                      &r.Coordinates,
	})
}

type IpGeoResult struct {
	Status   string `json:"status"`
	Country  string `json:"countryCode"`
	State    string `json:"regionName"`
	CityName string `json:"city"`
	Coordinates
}

func (ip *IpGeoResult) GetAddress() LocationReadableAddress {
	if ip.Status != "success" {
		return LocationReadableAddress{}
	}
	return LocationReadableAddress{
		CityName: ip.CityName,
		State:    ip.State,
		Country:  ip.Country,
	}
}

type FullAddress struct {
	LocationReadableAddress
	Coordinates
}

type LocationReadableLocalizedAddress struct {
	LocationReadableAddress
	LocalNames json.RawMessage `json:"local_names"`
}

type LocationReadableAddress struct {
	CityName string `json:"name"`
	State    string `json:"state,omitempty"`
	Country  string `json:"country"`
}

func (l *LocationReadableAddress) Key() string {
	var b strings.Builder
	b.Grow(len(l.CityName) + len(l.State) + len(l.Country) + 4)

	b.WriteString(l.CityName)
	b.WriteByte(',')

	if l.State != "" {
		b.WriteString(l.State)
		b.WriteByte(',')
	}

	b.WriteString(l.Country)

	return b.String()
}
func (l *LocationReadableAddress) WriteKey(b *strings.Builder) {
	b.WriteString(l.CityName)
	b.WriteByte(',')
	if l.State != "" {
		b.WriteString(l.State)
		b.WriteByte(',')
	}
	b.WriteString(l.Country)
}

type Coordinates struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func (c *Coordinates) Key() string {
	var b strings.Builder
	b.Grow(20)

	var buf [32]byte

	res := strconv.AppendFloat(buf[:0], c.Lat, 'f', 2, 64)
	b.Write(res)

	b.WriteByte(',')

	res = strconv.AppendFloat(buf[:0], c.Lon, 'f', 2, 64)
	b.Write(res)

	return b.String()
}
func (c *Coordinates) WriteKey(b *strings.Builder) {
	var buf [32]byte

	res := strconv.AppendFloat(buf[:0], c.Lat, 'f', 2, 64)
	b.Write(res)

	b.WriteByte(',')

	res = strconv.AppendFloat(buf[:0], c.Lon, 'f', 2, 64)
	b.Write(res)
}
