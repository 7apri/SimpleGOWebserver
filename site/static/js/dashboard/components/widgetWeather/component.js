import getData from "./getData.js";
import render from "./render.js";

let timerId = null;

async function main() {
    if (timerId) clearTimeout(timerId);
    let data, renderCurrent = true;
    try{
        data = await getData();
    } catch (error) {
        console.error("Weather update failed:", error);
    } finally {
        timerId = setTimeout(main, 590000);
        const activeWeekListItemEl = document.querySelector('.weather-info-week-list-item.active');
            if(activeWeekListItemEl) renderCurrent = activeWeekListItemEl.dataset.index === '0';
        render(data.weatherData,data.locationData.city,renderCurrent);
    }
}
export default main;