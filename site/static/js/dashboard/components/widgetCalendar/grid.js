import { toDateString } from "../../../util/misc.js";
import { changeDateInputContent } from "../widgetTodo/dateInput.js";
import { getTasksByDate } from "../widgetTodo/task.js";
import { changeCalendarDateSelectorDate } from "./dateSelector.js";

const dateToday = new Date();

/**
 * Manages the calendar grid UI, including date calculations, 
 * rendering 42-day views, and handling day selection.
 * @class
 */

class CalendarGrid {
    /** @type {HTMLLIElement[]} List of <li> elements representing calendar days */
    _liElements = null;
    /** @type {number|null} Current selected year */
    _selectedYear = null;
    /** @type {number|null} Current selected month (1-12) */
    _selectedMonth = null;
    /** @type {number|null} Current selected day of the month */
    _selectedDay = null;
    /** @type {number|null} Index (0-41) of the currently selected <li> element */
    _selectedDayElIndex = null;
    /** @type {Function|null} Callback triggered when the date changes */
    _refreshCallback = null;

    /** @returns {number|null} The currently selected day number */
    get selectedDay() { return this._selectedDay }

    /**
     * Gets or sets the date via an array.
     * @type {number[]} [year, month, day]
     */
    get fullDateArray() {
        return [this._selectedYear, this._selectedMonth, this._selectedDay];
    }
    set fullDateArray(value) {
        this.setDate(value);
    }

    /**
     * Gets or sets the date via a string (YYYY-MM-DD).
     * @type {string|null}
     */
    get fullDateString() {
        if (!this._selectedYear || !this._selectedMonth || !this._selectedDay) return null;
        return toDateString(this._selectedYear, this._selectedMonth, this._selectedDay);
    }
    set fullDateString(value) {
        this.setDate(value.split('-').map(Number));
    }

    /**
     * Updates the internal state and triggers a grid refresh if necessary.
     * @param {number[]} dateArray - [year, month, day]
     */
    setDate ([year, month, day]) {
        if(month === undefined) return;
        if( month < 1 || month > 12) month = 1;

        const dayMax = new Date(year, month, 0).getDate();
            if( day > dayMax ) day = dayMax;

        if(this._selectedMonth !== month || this._selectedYear !== year){

            this.updateGrid([year,month]);
            this.selectDay(toDateString(year,month,day));
        }
        else if(this._selectedDay !== day) this.selectDay(toDateString(year,month,day));

        [this._selectedYear, this._selectedMonth, this._selectedDay] = [year, month, day];

        this._refreshCallback && this._refreshCallback(this.fullDateString);  
    }

    /**
     * Creates the calendar grid and attaches click listeners.
     * @param {HTMLElement} parentEl - The <ul> or container for the grid.
     * @param {Function} [refreshCallback] - Runs when the date is updated.
     * @param {Function} [daySelectedCallback] - Runs when a day is clicked.
     */
    constructor(parentEl, refreshCallback, daySelectedCallback) {
        let finalHtml = "";
        for( let i = 0; i < 42; i++){
            finalHtml +=
            `
            <li class="calendar-day grid center pos-relative fill-width" data-index=${i}>
                <button type="button" class="day-btn grid center pos-relative" aria-pressed="false">
                    <span class="day-num" aria-hidden="true"></span>
                </button>
            </li>
            `
        }
        parentEl.innerHTML = finalHtml;
        this._liElements = [...parentEl.children];

        parentEl.addEventListener('click', (e) => {
            const calendarDay = e.target.closest('.calendar-day');
                if (!calendarDay || calendarDay.classList.contains('active')) return;
            this._daySelectedCallback && this._daySelectedCallback(calendarDay);
            this.fullDateString = calendarDay.dataset.date;
        });

        if(refreshCallback) this._refreshCallback = refreshCallback;
        if(daySelectedCallback) this._daySelectedCallback = daySelectedCallback;
    }

    /**
     * Visualizes the selection of a specific day and handles ARIA states.
     * @param {HTMLElement|string} day - The <li> element or a date string.
     */
    selectDay = (day) =>{
        if( typeof day === "string" ){
            day = this.findDayByDateString(day);
        }
        const rowNumber = Math.trunc(day.dataset.index / 7);
        
        if(this._selectedDayElIndex){
            const lastSelectedDay = this._liElements[this._selectedDayElIndex];

            lastSelectedDay.classList.remove('selected');
            lastSelectedDay.querySelector('button').setAttribute('aria-pressed', 'false');
        }
        
        const dayBtn = day.querySelector('button')
        day.classList.add('selected');
        dayBtn.setAttribute('aria-pressed', 'true');
        day.parentElement.matches(':focus-within') && dayBtn.focus();
        
        document.querySelectorAll('.active-week').forEach(calendarDay => calendarDay.classList.remove('active-week'));
        for( let i = rowNumber * 7; i < rowNumber * 7 + 7; i++){
            this._liElements[i].classList.add('active-week');
        }

        this._daySelectedCallback && this._daySelectedCallback(day);
        this._selectedDayElIndex = day.dataset.index;
    };

    /**
     * Main rendering logic. Populates the 42 cells with correct day numbers,
     * handles inactive days (prev/next month), and task indicators.
     * @param {number[]|string} yearMonth - [year, month] or date string.
     */
    updateGrid = (yearMonth) => {
        if(typeof yearMonth === "string" ){
            yearMonth = yearMonth.split('-').map(Number);
        }
        
        const [year, month] = yearMonth
        
        const firstDayIndex = new Date(year, month - 1, 1).getDay();
        const startOffset = firstDayIndex === 0 ? 6 : firstDayIndex - 1;

        const daysInMonth = new Date(year, month, 0).getDate();
        const daysInPrevMonth =  new Date(year, month -1, 0).getDate();

        const todayIndex = (dateToday.getMonth() + 1 === month && dateToday.getFullYear() === year) 
            ? (dateToday.getDate() + startOffset - 1)
            : null;

        const indexForecastMax = todayIndex !== null ? todayIndex + 7 : null;

        this._liElements.forEach((dayLi, index) => {
            let classesToAdd = [];
            let targetYear = year;
            let targetMonth = month;
            let dayNumber = index - startOffset + 1;
            
            const span = dayLi.querySelector('.day-num');
            const dayBtn = dayLi.querySelector("button");
            
            if (todayIndex !== null && index >= todayIndex && index <= indexForecastMax) {
                classesToAdd.push("weather-forecast");
                dayBtn.setAttribute("aria-controls","weather-day");
                
                if (index === todayIndex) classesToAdd.push("forecast-start");  
                if (index === indexForecastMax) classesToAdd.push("forecast-end");
            } 
            
            const weekIndex = index % 7;
            if(weekIndex === 0) classesToAdd.push('first');
            else if(weekIndex === 6) classesToAdd.push('last');
            
            
            if(dayNumber <= 0){
                dayNumber += daysInPrevMonth;
                classesToAdd.push('inactive');
                
                if( month === 1 ) {
                    targetMonth = 12
                    targetYear= year - 1
                } else targetMonth = month - 1;
                
            } else if(dayNumber > daysInMonth){
                dayNumber -= daysInMonth;
                classesToAdd.push('inactive');
                
                if (month === 12) {
                    targetMonth = 1;
                    targetYear = year + 1;
                } else targetMonth = month + 1;
                
            }

            
            const dateStr = `${targetYear}-${String(targetMonth).padStart(2, '0')}-${String(dayNumber).padStart(2, '0')}`;
            dayLi.dataset.date = dateStr;
            
            let ariaLabel;
            
            const daysTasks = getTasksByDate(dateStr);
            if(daysTasks.length > 0){
                if (daysTasks.every(task => task.completed)){
                    classesToAdd.push("completed");
                    ariaLabel = ",všechny úkoly splněny.";
                }
                else ariaLabel = ",máte úkoly.";
                
                classesToAdd.push("hasTasks");
            } else{ ariaLabel = ",bez úkolů."; }
            
            dayLi.classList.remove('inactive','hasTasks','weather-forecast');
            
            classesToAdd.length > 0 && dayLi.classList.add(...classesToAdd);
            
            ariaLabel = new Date(targetYear, targetMonth - 1, dayNumber).toLocaleDateString("default",{
                weekday: 'long',
                day: 'numeric',
                month: 'long',
                ...(dateToday.getFullYear() !== targetYear && {year: 'numeric'})
            }) + ariaLabel;

            if(todayIndex !== null && index === todayIndex){
                dayBtn.setAttribute("aria-current","date");
                ariaLabel = "dnes, " + ariaLabel;
            }

            dayBtn.setAttribute('aria-label', ariaLabel);
            span.textContent = dayNumber;
        });

    };

    /**
     * Updates the status (tasks/completion) for the currently selected day only.
     */
    updateSelectedDay = () =>{
        const daysTasks = getTasksByDate(toDateString(this._selectedYear,this._selectedMonth,this._selectedDay));
        const daysBtnEL =  this._liElements[this._selectedDayElIndex].querySelector('button');
        let ariaLabel = daysBtnEL.getAttribute('aria-label').split(',');

        if( daysTasks.length > 0 ){
            if( daysTasks.every(task => task.completed)){
                this._liElements[this._selectedDayElIndex].classList.add("completed");
                ariaLabel[ariaLabel.length-1] = "všechny úkoly splněny."
            }else {
                this._liElements[this._selectedDayElIndex].classList.remove("completed");
                this._liElements[this._selectedDayElIndex].classList.add("hasTasks");
                ariaLabel[ariaLabel.length-1] = "máte úkoly."
            }
        }else{
            this._liElements[this._selectedDayElIndex].classList.remove("hasTasks");
            ariaLabel[ariaLabel.length-1] = "bez úkolů."
        }
       daysBtnEL.setAttribute('aria-label',ariaLabel.join(','));
    };

    /**
     * Helper to find the DOM element corresponding to a date string.
     * @param {string} dateString - Format YYYY-MM-DD.
     * @returns {HTMLLIElement|undefined}
     */
    findDayByDateString = (dateString) =>{
        for(let i = 0; i < this._liElements.length; i++){
            if (this._liElements[i].dataset.date === dateString){
                return this._liElements[i];
            }
        };
    };
}

/** @type {CalendarGrid|null} Global instance of the active calendar grid */
let activeGrid = null;

/**
 * Initializes the calendar grid, binds it to the DOM, 
 * and sets up cross-widget communication callbacks.
 */
const setup = () =>{
    activeGrid = new CalendarGrid(document.getElementById("calendarGrid"),
    ( dateStr ) =>{
        const [year, month, day] = dateStr.split('-');
        
        changeCalendarDateSelectorDate(month,year);
        changeDateInputContent(dateStr);

    },(selectedDayEl) =>{
        if(selectedDayEl.classList.contains('weather-forecast')){
            const forecastDaySelected = new CustomEvent('calendar:forecastDaySelected', {
                detail: { 
                    index: selectedDayEl.dataset.index - document.querySelector('.forecast-start').dataset.index
                },
                bubbles: true 
            });
            selectedDayEl.parentElement.dispatchEvent(forecastDaySelected);
        }
    });
};

export {activeGrid};
export default setup;
