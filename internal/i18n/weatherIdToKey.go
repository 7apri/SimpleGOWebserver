package i18n

func getWeatherIdToKey() map[int16]string {
	return map[int16]string{
		// Group 2xx: Thunderstorm
		200: "thunderstorm_light_rain",
		201: "thunderstorm_rain",
		202: "thunderstorm_heavy_rain",
		210: "light_thunderstorm",
		211: "thunderstorm",
		212: "heavy_thunderstorm",
		221: "ragged_thunderstorm",
		230: "thunderstorm_light_drizzle",
		231: "thunderstorm_drizzle",
		232: "thunderstorm_heavy_drizzle",

		// Group 3xx: Drizzle
		300: "light_drizzle",
		301: "drizzle",
		302: "heavy_drizzle",
		310: "light_drizzle_rain",
		311: "drizzle_rain",
		312: "heavy_drizzle_rain",
		313: "shower_rain_drizzle",
		314: "heavy_shower_rain_drizzle",
		321: "shower_drizzle",

		// Group 5xx: Rain
		500: "light_rain",
		501: "moderate_rain",
		502: "heavy_rain",
		503: "very_heavy_rain",
		504: "extreme_rain",
		511: "freezing_rain",
		520: "light_shower_rain",
		521: "shower_rain",
		522: "heavy_shower_rain",
		531: "ragged_shower_rain",

		// Group 6xx: Snow
		600: "light_snow",
		601: "snow",
		602: "heavy_snow",
		611: "sleet",
		612: "light_shower_sleet",
		613: "shower_sleet",
		615: "light_rain_snow",
		616: "rain_snow",
		620: "light_shower_snow",
		621: "shower_snow",
		622: "heavy_shower_snow",

		// Group 7xx: Atmosphere
		701: "mist",
		711: "smoke",
		721: "haze",
		731: "sand_dust_whirls",
		741: "fog",
		751: "sand",
		761: "dust",
		762: "volcanic_ash",
		771: "squalls",
		781: "tornado",

		// Group 800: Clear
		800: "clear_sky",

		// Group 80x: Clouds
		801: "few_clouds",
		802: "scattered_clouds",
		803: "broken_clouds",
		804: "overcast_clouds",
	}
}
