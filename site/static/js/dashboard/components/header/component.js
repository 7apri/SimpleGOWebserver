import { capitalize } from "../../../util/misc.js";
/**
 * Renders the lopcation into the header provided.
 * Or by default into the element with the id location-name.
 * 
 * @param {string} city - The city to render.
 * @param {HTMLElement} element - The element to render into.
 * 
 * @returns {boolean} Either true or false depens on how it went.
 */
const renderHeaderCity = (city, element = document.getElementById("location-name")) =>  {
    try{
        if( city.length <= 0) city = "Failed"

        element.textContent = city;

        return true;
    }
    catch(err){
        console.error("Got an error: [" + err  + "] rendering the city into: " + element);
        return false;
    }
}

/**
 * Renders the update date into the DOM element thats provided.
 * Or by default into the element with the id update-date.
 *
 * @param {string} date - The day's date to render.
 * @param {HTMLElement} element - The element to render into.
 * 
 * @returns {boolean} Either true or false depens on how it went.
 */
const renderHeaderUpdateDate = (date, element = document.getElementById("update-date")) =>  {
    try{
        if( date.length <= 0) date = "Neco se pokazilo"

        element.textContent = capitalize(date);
        
        return true;
    }
    catch(err){
        console.error("Got an error: [" + err  + "] rendering the date into: " + element);
        return false;
    }
}

/**
 * Renders the header's city and date into the el provided.
 * Or by default into the header.
 *
 * @param {string} updateDate - The day's date to render.
 * @param {string} city - The city to render.
 * @param {HTMLElement} element - The element to render into.
 * 
 * @returns {boolean} Either true or false depens on how it went.
 */
const renderHeader = (updateDate= "" , city= "", element = document.querySelector("header")) =>  {
    try{
        city !== "" && renderHeaderCity(city);
        updateDate !== "" && renderHeaderUpdateDate(updateDate);
        
        return true;
    }
    catch(err){
        console.error("Got an error rendering the header: " + err);
        return false;
    }
}

export {renderHeaderUpdateDate,renderHeaderCity};
export default renderHeader;
