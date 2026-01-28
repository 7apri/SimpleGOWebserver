package location

import (
	"strconv"
	"strings"
)

type GeoResult struct {
	LocalNames map[string]string `json:"local_names"`
	FullAddress
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

type LocationReadableAddress struct {
	CityName string `json:"name"`
	State    string `json:"state,omitempty"`
	Country  string `json:"country"`
}

func (l *LocationReadableAddress) Key() string {
	var b strings.Builder
	b.Grow(len(l.CityName) + len(l.State) + len(l.Country) + 4)
	b.WriteString("a:")

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
	b.WriteString("a:")
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

	b.WriteString("c:")

	res := strconv.AppendFloat(buf[:0], c.Lat, 'f', 2, 64)
	b.Write(res)

	b.WriteByte(',')

	res = strconv.AppendFloat(buf[:0], c.Lon, 'f', 2, 64)
	b.Write(res)

	return b.String()
}
func (c *Coordinates) WriteKey(b *strings.Builder) {
	var buf [32]byte

	b.WriteString("c:")

	res := strconv.AppendFloat(buf[:0], c.Lat, 'f', 2, 64)
	b.Write(res)

	b.WriteByte(',')

	res = strconv.AppendFloat(buf[:0], c.Lon, 'f', 2, 64)
	b.Write(res)
}
