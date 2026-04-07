import renderDay,{renderCurrent} from "./renderDay.js";
import { getLocaleDateString } from "../../../util/toLocalFromUnix.js";
import { stupne, capitalize } from "../../../util/misc.js";
/**
 * Renders the week list into the DOM element thats provided.
 * Or by default into the element with the id week-list.
 *
 * @param {Array<Object>} dailyArray - The array of daily data to render from.
 * @param {Object} currentData - The day's data to render from.
 * @param {string} timezone - The time zone used for converting the unix time
 * @param {HTMLUListElement} element - The element to render into.
 * 
 * 
 * @returns {boolean} Either true or false depens on how it went.
 */
const renderWeekList = (dailyArray, currentData, timezone, element = document.getElementById("week-list")) => {
    try{
        const weekListItemElements = element.querySelectorAll('.weather-info-week-list-item');
        const createAriaLabel = (day, max, min) => {
            const dayName = capitalize(getLocaleDateString(day.dt, timezone, { weekday: 'long' }));
            return `${dayName}: ${day.weather[0].description}, maximálně ${max} ${stupne(max)}, minimálně ${min} ${stupne(min)}`;
        };

        if(weekListItemElements.length <= 0){
            let finalHtml = "";
            for( let i = 0; i < dailyArray.length; i++){
                const day = dailyArray[i];
                const maxTemp = Math.round(day.temp.max);
                const minTemp = Math.round(day.temp.min);
                finalHtml +=    `<li class="weather-info-week-list-item flex column center ${i === 0 ? "active" : ""}" data-index="${i}">
                                    <button class="fill" aria-label="${createAriaLabel(day, maxTemp, minTemp)}" aria-current="${i === 0}">
                                        <p class="weather-info-week-day" aria-hidden="true" >${getLocaleDateString(day.dt ,timezone)}</p>
                                        <svg class="icon" aria-hidden="true">
                                            <use href="./assets/iconBundle.svg#${day.weather[0].icon.slice(0,2)}"></use>
                                        </svg>
                                        <p class="weather-info-week-temp" aria-hidden="true">${maxTemp}/${minTemp}</p>
                                    </button>
                                </li>`
            }   
            element.innerHTML = finalHtml;
            const leftBtn = element.parentElement.querySelector(".left");
            const rightBtn = element.parentElement.querySelector(".right");

            const updateButtonVisibility = () => {
                const scrollLeft = element.scrollLeft;

                const canScrollLeft = scrollLeft <= 20;
                const canScrollRight = scrollLeft >= element.scrollWidth - element.clientWidth - 20;

                leftBtn.style.opacity = canScrollLeft ? "0" : "0.3";
                leftBtn.style.pointerEvents = canScrollLeft ? "none" : "auto";
                leftBtn.setAttribute('aria-hidden', canScrollLeft);
                leftBtn.setAttribute('tabindex', canScrollLeft ? "-1" : "0");

                rightBtn.style.opacity = canScrollRight ? "0" : "0.3";
                rightBtn.style.pointerEvents = canScrollRight ? "none" : "auto";
                rightBtn.setAttribute('aria-hidden', canScrollRight);
                rightBtn.setAttribute('tabindex', canScrollRight ? "-1" : "0");
            };

            element.addEventListener('scroll', updateButtonVisibility);
            element.addEventListener('scroll', updateButtonVisibility);
            const resizeObserver = new ResizeObserver(() => {
                updateButtonVisibility();
            });

            resizeObserver.observe(element);
            updateButtonVisibility();

            leftBtn.addEventListener('click', () => {
                element.scrollBy({
                    left: -(element.scrollWidth - element.clientWidth),
                    behavior: 'smooth'
                });
            });
            rightBtn.addEventListener('click', () => {
                element.scrollBy({
                    left: element.scrollWidth - element.clientWidth,
                    behavior: 'smooth'
                });
            });

            const renderIndex = ( index , day) =>{
                if( !day ) day = [...element.querySelectorAll('.weather-info-week-list-item')].filter( li => li.dataset.index == index)[0];
                if( !day ) return;

                index === 0 ? renderCurrent(currentData,dailyArray[0].temp,timezone) : renderDay(dailyArray[index],timezone);

                const currentActive = element.querySelector('.weather-info-week-list-item.active');
                if (currentActive){
                    currentActive.classList.remove('active')
                    currentActive.querySelector('button').setAttribute('aria-current','false');
                }
                day.classList.add('active');
                day.querySelector('button').setAttribute('aria-current','true');
            }

            element.addEventListener('click', (e) => {
                const day = e.target.closest('.weather-info-week-list-item');
                if (!day) return;
                const index = parseInt(day.dataset.index);

                renderIndex(index, day);
            });

            const calendarEl = document.getElementById("calendarGrid");

            calendarEl.addEventListener('calendar:forecastDaySelected', (e) => {
                renderIndex(e.detail.index);
            })
        } else {
            weekListItemElements.forEach(( weekListElement, i) =>{
                const day = dailyArray[i];
                if(!day) return;
                const maxTemp = Math.round(day.temp.max);
                const minTemp = Math.round(day.temp.min);
                const dayDateStr = getLocaleDateString(day.dt ,timezone);
                const weatherIconCode = day.weather[0].icon.slice(0,2);

                let updateAriaLabel = false;
                
                const weekLeTempEL = weekListElement.querySelector('.weather-info-week-temp');
                const prevTemps = weekLeTempEL.textContent.split('/').map(Number);
                if( maxTemp !== prevTemps[0] || minTemp !== prevTemps[1] ){
                    weekLeTempEL.textContent = `${maxTemp}/${minTemp}`;
                    updateAriaLabel = true;
                }

                const weekLeDayEL = weekListElement.querySelector('.weather-info-week-day');
                const prevDayDateStr = weekLeDayEL.textContent;
                if(dayDateStr !== prevDayDateStr){
                    weekLeDayEL.textContent = dayDateStr;
                    updateAriaLabel = true;
                }
                
                const weekLeIconUseEl = weekListElement.querySelector('.icon use');
                const prevWeatherIconCode = weekLeIconUseEl.getAttribute('href').split('#')[1];
                if(weatherIconCode !== prevWeatherIconCode){
                    weekLeIconUseEl.setAttribute('href',`./assets/icons/iconBundle.svg#${weatherIconCode}`)
                    updateAriaLabel = true;
                }

                if(updateAriaLabel) weekListElement.querySelector('button').setAttribute('aria-label',createAriaLabel(day, maxTemp, minTemp));
            })
        }
        return true;
    }
    catch(err){
        console.error("Got an error rendering the week: "+ err +" on element: " + element);
        return false;
    }
}

export default renderWeekList;
