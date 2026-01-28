package util

import (
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

// 1. Table-Driven Unit Test for ParseGenericQuery
// This is the Go standard for testing multiple inputs.
func TestParseGenericQuery(t *testing.T) {
	type coords struct {
		Lat float64
		Lon float64
	}

	mapper := func(row []string) coords {
		lat, _ := strconv.ParseFloat(row[0], 64)
		lon, _ := strconv.ParseFloat(row[1], 64)
		return coords{lat, lon}
	}

	tests := []struct {
		name     string
		latStr   string
		lonStr   string
		expected []coords
	}{
		{
			name:   "Standard Multi-Value",
			latStr: "10.5, 20.5",
			lonStr: "30.5, 40.5",
			expected: []coords{
				{10.5, 30.5},
				{20.5, 40.5},
			},
		},
		{
			name:     "Mismatched Lengths (should use minLen)",
			latStr:   "1.0, 2.0, 3.0",
			lonStr:   "4.0, 5.0",
			expected: []coords{{1.0, 4.0}, {2.0, 5.0}},
		},
		{
			name:     "Empty Input",
			latStr:   "",
			lonStr:   "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseGenericQuery(mapper, tt.latStr, tt.lonStr)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseGenericQuery() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// 2. Fuzz Test: The "Chaos Monkey"
// This will try to crash your parser with random characters, emojis, and null bytes.
func FuzzParseGenericQuery(f *testing.F) {
	f.Add("10.0,20.0", "30.0,40.0")

	f.Fuzz(func(t *testing.T, a string, b string) {
		ParseGenericQuery(func(row []string) int {
			return len(row)
		}, a, b)
	})
}

// 3. Benchmark: Measuring the "Tax"
// We want to see how many nanoseconds it takes and how many heap allocations happen.
func BenchmarkParseGenericQuery(b *testing.B) {
	lat := "10.0,20.0,30.0,40.0,50.0"
	lon := "60.0,70.0,80.0,90.0,100.0"

	mapper := func(row []string) float64 {
		f, _ := strconv.ParseFloat(row[0], 64)
		return f
	}

	b.ReportAllocs()
	for b.Loop() {
		ParseGenericQuery(mapper, lat, lon)
	}
}

// 4. Testing HTTP Handlers (The SendJson helper)
func TestSendJson(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"city": "Osaka"}

	SendJson(w, 200, data)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content type application/json, got %s", contentType)
	}
}
