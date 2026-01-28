package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/location"
)

type resp struct {
	time time.Duration
	id   int
}

func main() {
	/*
		locations := []location.LocationReadableAddress{
			{CityName: "prague", Country: "CZ"}, {CityName: "brno", Country: "CZ"},
			{CityName: "berlin", Country: "DE"}, {CityName: "munich", Country: "DE"},
			{CityName: "paris", Country: "FR"}, {CityName: "lyon", Country: "FR"},
			{CityName: "london", Country: "GB"}, {CityName: "manchester", Country: "GB"},
			{CityName: "madrid", Country: "ES"}, {CityName: "barcelona", Country: "ES"},
			{CityName: "rome", Country: "IT"}, {CityName: "milan", Country: "IT"},
			{CityName: "vienna", Country: "AT"}, {CityName: "salzburg", Country: "AT"},
			{CityName: "warsaw", Country: "PL"}, {CityName: "krakow", Country: "PL"},
			{CityName: "amsterdam", Country: "NL"}, {CityName: "rotterdam", Country: "NL"},
			{CityName: "brussels", Country: "BE"}, {CityName: "antwerp", Country: "BE"},
			{CityName: "stockholm", Country: "SE"}, {CityName: "gothenburg", Country: "SE"},
			{CityName: "oslo", Country: "NO"}, {CityName: "bergen", Country: "NO"},
			{CityName: "helsinki", Country: "FI"}, {CityName: "tampere", Country: "FI"},
			{CityName: "lisbon", Country: "PT"}, {CityName: "porto", Country: "PT"},
			{CityName: "athens", Country: "GR"}, {CityName: "thessaloniki", Country: "GR"},
			{CityName: "budapest", Country: "HU"}, {CityName: "debrecen", Country: "HU"},
			{CityName: "dublin", Country: "IE"}, {CityName: "cork", Country: "IE"},
			{CityName: "copenhagen", Country: "DK"}, {CityName: "aarhus", Country: "DK"},
			{CityName: "zurich", Country: "CH"}, {CityName: "geneva", Country: "CH"},
			{CityName: "bratislava", Country: "SK"}, {CityName: "kosice", Country: "SK"},
			{CityName: "sofia", Country: "BG"}, {CityName: "plovdiv", Country: "BG"},
			{CityName: "bucharest", Country: "RO"}, {CityName: "cluj", Country: "RO"},
			{CityName: "zagreb", Country: "HR"}, {CityName: "split", Country: "HR"},
			{CityName: "belgrade", Country: "RS"}, {CityName: "lubna", Country: "CZ"},
			{CityName: "tokyo", Country: "JP"}, {CityName: "osaka", Country: "JP"},
			{CityName: "seoul", Country: "KR"}, {CityName: "busan", Country: "KR"},
			{CityName: "beijing", Country: "CN"}, {CityName: "shanghai", Country: "CN"},
			{CityName: "bangkok", Country: "TH"}, {CityName: "phuket", Country: "TH"},
			{CityName: "singapore", Country: "SG"}, {CityName: "jakarta", Country: "ID"},
			{CityName: "mumbai", Country: "IN"}, {CityName: "delhi", Country: "IN"},
			{CityName: "sydney", Country: "AU"}, {CityName: "melbourne", Country: "AU"},
			{CityName: "auckland", Country: "NZ"}, {CityName: "wellington", Country: "NZ"},
			{CityName: "new-york", Country: "US"}, {CityName: "los-angeles", Country: "US"},
			{CityName: "chicago", Country: "US"}, {CityName: "houston", Country: "US"},
			{CityName: "old-toronto", Country: "CA"}, {CityName: "vancouver", Country: "CA"},
			{CityName: "mexico-city", Country: "MX"}, {CityName: "cancun", Country: "MX"},
			{CityName: "sao-paulo", Country: "BR"}, {CityName: "rio-de-janeiro", Country: "BR"},
			{CityName: "buenos-aires", Country: "AR"}, {CityName: "santiago", Country: "CL"},
			{CityName: "bogota", Country: "CO"}, {CityName: "lima", Country: "PE"},
			{CityName: "cairo", Country: "EG"}, {CityName: "alexandria", Country: "EG"},
			{CityName: "cape-town", Country: "ZA"}, {CityName: "johannesburg", Country: "ZA"},
			{CityName: "nairobi", Country: "KE"}, {CityName: "casablanca", Country: "MA"},
			{CityName: "dubai", Country: "AE"}, {CityName: "abu-dhabi", Country: "AE"},
			{CityName: "istanbul", Country: "TR"}, {CityName: "ankara", Country: "TR"},
			{CityName: "riyadh", Country: "SA"}, {CityName: "jeddah", Country: "SA"},
			{CityName: "tel-aviv", Country: "IL"}, {CityName: "jerusalem", Country: "IL"},
			{CityName: "kyiv", Country: "UA"}, {CityName: "lviv", Country: "UA"},
			{CityName: "reykjavik", Country: "IS"}, {CityName: "luxembourg", Country: "LU"},
			{CityName: "tallinn", Country: "EE"}, {CityName: "riga", Country: "LV"},
			{CityName: "vilnius", Country: "LT"}, {CityName: "valletta", Country: "MT"},
		}*/

	const totalRequests = 500
	const concurrentWorkers = 100

	var httpCl = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        totalRequests,
			MaxIdleConnsPerHost: totalRequests,
			DisableKeepAlives:   false,
			ForceAttemptHTTP2:   true,
		},
		Timeout: time.Second * 1,
	}

	respChan := make(chan resp, totalRequests)
	jobs := make(chan int, totalRequests)
	var wg sync.WaitGroup

	for range concurrentWorkers {
		go func() {
			for id := range jobs {
				u := "http://localhost/api/location?city=nairobi&country=KE&state=-" // "http://localhost/api/location?city=prague,brno,berlin,munich,paris,lyon,london,manchester,madrid,barcelona,rome,milan,vienna,salzburg,warsaw,krakow,amsterdam,rotterdam,brussels,antwerp,stockholm,gothenburg,oslo,bergen,helsinki,tampere,lisbon,porto,athens,thessaloniki,budapest,debrecen,dublin,cork,copenhagen,aarhus,zurich,geneva,bratislava,kosice,sofia,plovdiv,bucharest,cluj,zagreb,split,belgrade,lubna,tokyo,osaka,seoul,busan,beijing,shanghai,bangkok,phuket,singapore,jakarta,mumbai,delhi,sydney,melbourne,auckland,wellington,new-york,los-angeles,chicago,houston,old-toronto,vancouver,mexico-city,cancun,sao-paulo,rio-de-janeiro,buenos-aires,santiago,bogota,lima,cairo,alexandria,cape-town,johannesburg,nairobi,casablanca,dubai,abu-dhabi,istanbul,ankara,riyadh,jeddah,tel-aviv,jerusalem,kyiv,lviv,reykjavik,luxembourg,tallinn,riga,vilnius,valletta&country=CZ,CZ,DE,DE,FR,FR,GB,GB,ES,ES,IT,IT,AT,AT,PL,PL,NL,NL,BE,BE,SE,SE,NO,NO,FI,FI,PT,PT,GR,GR,HU,HU,IE,IE,DK,DK,CH,CH,SK,SK,BG,BG,RO,RO,HR,HR,RS,CZ,JP,JP,KR,KR,CN,CN,TH,TH,SG,ID,IN,IN,AU,AU,NZ,NZ,US,US,US,US,CA,CA,MX,MX,BR,BR,AR,CL,CO,PE,EG,EG,ZA,ZA,KE,MA,AE,AE,TR,TR,SA,SA,IL,IL,UA,UA,IS,LU,EE,LV,LT,MT&state=-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-,-"
				t := time.Now()
				response, err := httpCl.Get(u)
				if err != nil {
					return
				}
				ti := time.Since(t)

				var result []location.GeoResult
				err = json.NewDecoder(response.Body).Decode(&result)
				if err != nil {
					return
				}
				respChan <- resp{
					time: ti,
					id:   id,
				}
				wg.Done()
				response.Body.Close()
			}
		}()
	}

	for i := range totalRequests {
		wg.Add(1)
		jobs <- i
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(respChan)
	}()

	var lowest *resp
	var highest *resp

	var responses = make([]*resp, 0, totalRequests)

	st := time.Now()
	for loc := range respChan {
		if lowest == nil || loc.time < lowest.time {
			lowest = &loc
		}
		if highest == nil || loc.time > highest.time {
			highest = &loc
		}
		responses = append(responses, &loc)
	}
	BubbleSort(responses)
	for _, resp := range responses {
		slog.Info("responce", "time", resp.time, "id", resp.id)
	}
	slog.Info("stats", "lowest", lowest.time, "highest", highest.time, "total", time.Since(st))
}

func BubbleSort(slice []*resp) {
	n := len(slice)
	for i := range n {
		swapped := false

		for j := 0; j < n-i-1; j++ {
			if slice[j].time > slice[j+1].time {
				slice[j], slice[j+1] = slice[j+1], slice[j]
				swapped = true
			}
		}
		if !swapped {
			break
		}
	}
}
