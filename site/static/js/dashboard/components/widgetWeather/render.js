import { renderHeaderCity } from "../header/component.js";
import {renderCurrent} from "./renderDay.js";
import renderWeekList from "./renderWeekList.js";

/**
 * Renders the widget into the DOM element thats provided.
 * Or by default into the element with the id weather.
 *
 * @param {Object} weatherData - The weather data to render from.
 * @param {string} city - The city to render in the header.
 * @param {boolean} updateCurrent - Does it render in the current day?
 * @param {HTMLUListElement} element - The element to render into.
 * @returns {boolean} Either true or false depens on how it went.
 */
const render = (weatherData, city, updateCurrent  = true, element = document.getElementById("weather")) => {
    try{
        const timezone = weatherData.timezone
        
        renderHeaderCity(city);

        renderWeekList(weatherData.daily,weatherData.current,timezone);
        
        
        updateCurrent && renderCurrent(weatherData.current,weatherData.daily[0].temp, timezone);
        
        return true;
    }
    catch(err){
        console.error("Got an error rendering the week: "+ err +" on element: " + element);
        return false;
    }
}
export default render;
