import { renderHeaderUpdateDate } from "../header/component.js";
import renderGraph from "./renderGraph.js";
import { getLocaleDateString,getLocaleTimeString } from "../../../util/toLocalFromUnix.js";
import { getTimestamp } from "../../../util/localStorage.js";
import { stupne } from "../../../util/misc.js";


/**
 * Renders the current weather into the DOM element thats provided.
 * Or by default into the element with the id weather-day.
 *
 * @param {Object} currentData - The day's data to render from.
 * @param {Object} tempsCurrent - The day's temps to render from.
 * @param {string} timezone - The time zone used for converting the unix time
 * @param {HTMLDListElement} element - The element to render into.
 * 
 * @returns {boolean} Either true or false depens on how it went.
 */
const renderCurrent = (currentData, tempsCurrent , timezone , element = document.getElementById("weather-day")) =>  {
    try{
        const updateDate = getLocaleTimeString(getTimestamp('weather-current'),timezone,{
            hour: '2-digit',
            minute: '2-digit',
        });

        renderHeaderUpdateDate("Dnes " + updateDate);

        element.querySelector("h2.sr-only").textContent = "Aktuální počasí a předpověď";

        const tempElement = element.querySelector(".weather-day-temp");
        const srTempElement = element.querySelector(".weather-day-center .sr-only");
            currentData.temp = Math.round(currentData.temp);
            srTempElement.textContent = `Je ${currentData.weather[0].description} a ${currentData.temp} ${stupne(currentData.temp)}.`;
            tempElement.textContent = Math.abs(currentData.temp);
            if(currentData.temp < 0) tempElement.classList.add('cold');
            else tempElement.classList.remove('cold');
        element.querySelector(".weather-day-condition")
            .textContent = currentData.weather[0].description;
        
        const dayDetails = 
            Array.from(
        element.querySelector(".weather-day-details").children
            ).entries()
        for (const [index, childElement] of dayDetails){
            const textElement = childElement.querySelector(".weather-day-detail-text");
            if(!textElement) continue;
            switch (index){
                case 0:
                    textElement
                        .textContent = currentData.pressure;
                    break;
                case 1:
                    textElement
                        .textContent = Math.ceil(currentData.wind_speed);
                    break;
                case 2:
                    textElement
                        .textContent = currentData.humidity;
                    break;
                default:
                    console.log("A weather detail out of bounds will not render nothin i: " + index);
                    break;
            }
        }

        renderGraph([tempsCurrent.morn, tempsCurrent.day, tempsCurrent.eve, tempsCurrent.night]);

        return true;
    }
    catch(err){
        console.error("Got an error rendering the day: "+ err +" on element: " + element);
        return false;
    }
}

/**
 * Renders the day into the DOM element thats provided.
 * Or by default into the element with the id weather-day.
 *
 * @param {Object} dayData - The day's data to render from.
 * @param {string} timezone - The time zone used for converting the unix time
 * @param {HTMLDListElement} element - The element to render into.
 * 
 * @returns {boolean} Either true or false depens on how it went.
 */
const renderDay = (dayData, timezone ,element = document.getElementById("weather-day")) =>  {
    try{
        renderHeaderUpdateDate(getLocaleDateString(dayData.dt,timezone,{
            weekday: 'long', 
            day: 'numeric',
            month: 'short'
        },));

        const getEditedDateStr = (dateStr) =>{
             const dnyNa = {
                "pondělí": "na pondělí",
                "úterý": "na úterý",
                "středa": "na středu",
                "čtvrtek": "na čtvrtek",
                "pátek": "na pátek",
                "sobota": "na sobotu",
                "neděle": "na neděli"
            };
            let temp = dateStr.split(' ');
            temp[0] = dnyNa[temp[0]];

            return temp.join(' ')
        }

        element.querySelector("h2.sr-only").textContent =`Předpověď ${getEditedDateStr(getLocaleDateString(dayData.dt,timezone,{weekday: 'long', day: 'numeric', month: 'short' }))} a další dny v menu`;

        const srTempElement = element.querySelector(".weather-day-center .sr-only");
        const tempElement = element.querySelector(".weather-day-temp");
            dayData.temp.day = Math.round(dayData.temp.day);
            tempElement.textContent = Math.abs(dayData.temp.day);
            srTempElement.textContent = `Bude ${dayData.weather[0].description}.Teplota ve dne ${dayData.temp.day} ${stupne(dayData.temp.day)}.`;
            if(dayData.temp.day < 0) tempElement.classList.add('cold');
            else tempElement.classList.remove('cold');
        element.querySelector(".weather-day-condition")
            .textContent = dayData.weather[0].description;
        
        
        const dayDetails = 
            Array.from(
        element.querySelector(".weather-day-details").children
            ).entries()
        for (const [index, childElement] of dayDetails){
            const textElement = childElement.querySelector(".weather-day-detail-text");
            if(!textElement) continue;
            switch (index){
                case 0:
                    textElement
                        .textContent = dayData.pressure;
                    break;
                case 1:
                    textElement
                        .textContent = Math.round(dayData.wind_speed);
                    break;
                case 2:
                    textElement
                        .textContent = dayData.humidity;
                    break;
                default:
                    console.log("A weather detail out of bounds will not render nothin i: " + index);
                    break;
            }
        }

        const temps = dayData.temp;

        renderGraph([temps.morn, temps.day, temps.eve, temps.night],);

        return true;
    }
    catch(err){
        console.error("Got an error rendering the day: "+ err +" on element: " + element);
        return false;
    }
}

export { renderCurrent };
export default renderDay;
