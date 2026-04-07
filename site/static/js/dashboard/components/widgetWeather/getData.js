/**
 * Fetches geolocation data (city,latitute,lon) based on the client's IP address
 * using the ipapi.co service.
 *
 * @async
 * @param {string} userLang The lang passed to the api. 
 * @returns {Promise<Object | null>} A Promise that resolves to an object containing
 * location details, or null if the fetch fails.
 * @returns {string} return.city - The detected city name.
 * @returns {number} return.latitude - The latitude coordinate.
 * @returns {number} return.longitude - The longitude coordinate.
 */
export const getLocationFromIP = async ( userLang )=> {
    const res = await fetch(`https://ipapi.co/json/?language=${userLang}`);
    if (!res.ok) throw new Error('Network response was not ok: ' + res.status);
    const data = await res.json();

    return {
        city: data.city,
        latitude: data.latitude,
        longitude: data.longitude,
        source: 'ip'
    }
}
/**
 * Gets the current position from the browser and converts coords to a city name.
 * 
 * @param {string} userLang The lang passed to the api. 
 * 
 * @returns {Promise<Object>} Unified location object (similar to IP-API format).
 * @returns {number} return.longitude - The longitude coordinate.
 * @returns {number} return.latitude - The latitude coordinate.
 * @returns {string} return.city - The detected city name.
 * @returns {string} return.source - The source.
 */
export const getLocationFromBrowser = async (userLang) => {
    return new Promise((resolve, reject) => {
        
        navigator.geolocation.getCurrentPosition(async (pos) => {
            const { latitude, longitude } = pos.coords;
            try {

                const res = await fetch(
                    `https://api.bigdatacloud.net/data/reverse-geocode-client?latitude=${latitude}&longitude=${longitude}&localityLanguage=${userLang}`
                );
                
                if (!res.ok) throw new Error("Geocoding failed");
                const data = await res.json();

                resolve({
                    latitude,
                    longitude,
                    city: data.city || data.locality || "Unknown Location",
                    source: 'gps'
                });

            } catch (error) {
                console.warn("Could not find city name, returning coords only.", error);
                resolve({ 
                    latitude, 
                    longitude, 
                    city:  userLang === 'cz' ? "Tam kde ses" : "Local Location",
                    source: 'gps-nocity' 
                });
            }

        }, (err) => {
            reject(err); 
        }, {
            enableHighAccuracy: true,
            timeout: 5000,
            maximumAge: 0
        });
    });
};
/**
 * Abstracts location fetching by attempting Browser GPS first, 
 * then falling back to IP-based geolocation.
 *
 * @async
 * @returns {Promise<Object | null>} Resolves to an object with city, lat, lon, and source.
 * @returns {number} return.longitude - The longitude coordinate.
 * @returns {number} return.latitude - The latitude coordinate.
 * @returns {string} return.city - The detected city name.
 * @returns {string} return.source - The source.
 */
export const getLocation = async () => {
    const userLang = navigator.language.split('-')[0] || 'en';

    const result = await getLocationFromBrowser(userLang).catch(() => {
        return getLocationFromIP(userLang);
    });

    return result; 
};

/**
 * Fetches comprehensive weather data for a specific location using the OpenWeatherMap
 * One Call API (3.0).
 *
 * @async
 * @param {number} latitude - The latitude of the desired location.
 * @param {number} longitude - The longitude of the desired location.
 * @param {Array<string>} queryOptions - Optional params to pass to the api.
 * @returns {Promise<Object | undefined>} A Promise that resolves to the full OpenWeatherMap
 * response object (including current, daily, and hourly forecasts), or undefined on failure.
 */
export const getWeatherData = async (latitude, longitude, queryOptions = [] )=> {
    // REPLACE_ME_PLEASE: Insert your OpenWeatherMap API key here
    const apiKey = "YOUR-API-KEY-HERE";
        if(apiKey === "YOUR-API-KEY-HERE"){
            console.warn('Weather API key is missing! Please add it in getData.js');
            return null;
        }
    
    try{
        const response = await fetch(`https://api.openweathermap.org/data/3.0/onecall?lat=${latitude}&lon=${longitude}&appid=${apiKey}&lang=cz&` + queryOptions.join('&'));
        if(!response.ok) throw Error("API return not ok: " + response.status);
        const data = await response.json();
        return data;
    }
    catch(error){
        console.error("Error fetching weather data:", error);
    }
}

import getLocalStorage from "../../../util/localStorage.js";

/**
 * Fetches the client's current location, then uses those coordinates to fetch 
 * comprehensive weather data. This function supports using dummy data for 
 * development and testing purposes via the options object.
 *
 * @async
 * @param {Object} options - Configuration options for data fetching.
 * @param {boolean} [options.useDummyPos=false] - If true, uses pre-defined 
 * dummy location data instead of calling getLocationFromIP().
 * @param {boolean} [options.useDummyData=false] - If true, uses pre-defined 
 * dummy weather data instead of calling getWeatherData().
 * @returns {Promise<Object>} A Promise that resolves to an object containing 
 * the data results.
 * @returns {Object | null} return.locationData - The fetched location object or
 * the dummyPos object. Is null if the geolocation fetch failed.
 * @returns {Object | null} return.weatherData - The fetched weather object or 
 * the dummyWeatherData object. Is null if locationData was null, preventing the fetch.
 */
const getData  = async ( options = {useDummyPos:false,useDummyData:false} )=>{

    let locationData = null;
    let weatherData = null;

    if(options["useDummyPos"] || options["useDummyData"]){
        const {dummyPos,dummyWeatherData} = await import("./dummyData.js");

        if(options["useDummyPos"]) locationData =  dummyPos;

        if(options["useDummyData"]) weatherData = dummyWeatherData;
    }

    if(!locationData) locationData = await getLocalStorage('location',getLocation,180000);
    
    if (!locationData) {
        console.error("Could not get location. Cannot fetch weather data.");
        return {locationData, weatherData : null}; 
    }

    if (!weatherData) {
        const isLocationValid = (currentLocation) => (parsed) => {
            if(!(parsed.data.lat && parsed.data.lon)) return false;
            const latMatch = currentLocation.latitude.toFixed(2) === parsed.data.lat.toFixed(2);
            const lonMatch = currentLocation.longitude.toFixed(2) === parsed.data.lon.toFixed(2);
            return latMatch && lonMatch;
        };

        const dailyCheck = (currentLocation) => (parsed) => {
            if (!isLocationValid(currentLocation)(parsed)) return false;
            
            const savedDate = new Date(parsed.timestamp);
            const today = new Date();

            return (
                savedDate.getDate() === today.getDate() &&
                savedDate.getMonth() === today.getMonth() &&
                savedDate.getFullYear() === today.getFullYear()
            );
        };
        
        const [current, daily] = await Promise.all([
            getLocalStorage('weather-current', () => getWeatherData(locationData.latitude, locationData.longitude, ["units=metric","exclude=minutely,hourly,daily"]), 600000, isLocationValid(locationData)),
            getLocalStorage('weather-daily', () => getWeatherData(locationData.latitude, locationData.longitude, ["units=metric","exclude=minutely,hourly,current"]), 86400000,dailyCheck(locationData))
        ]);

        weatherData = { ...daily, ...current};
    }

    return {locationData, weatherData}; 
}
export default getData;
