/**
 * Module for managing the calendar date selection UI and logic.
 * Handles month/year display and navigation via click or long-press.
 * @module CalendarDateSelector
 */

import { capitalize } from "../../../util/misc.js";
import { activeGrid } from "./grid.js";

/** @type {HTMLElement|null} */
const monthEl = document.getElementById('monthDisplayCalendar');
/** @type {HTMLElement|null} */
const yearEl = document.getElementById('yearDisplayCalendar');

/** * Ref to the active interval for the long-press func.
 * @type {ReturnType<typeof setInterval>|null} 
 */
let holdTimer = null;

/**
 * Updates the UI elements to display the specified month and year.
 * @param {number} month - The month number (1-12).
 * @param {number} year - The full year (e.g., 2026).
 */
const changeCalendarDateSelectorDate = (month, year) => {
    const date = new Date(year, month - 1, 1);

    monthEl.textContent = capitalize(new Intl.DateTimeFormat('default', { month: 'long' }).format(date));
    yearEl.textContent = year;
};

/**
 * Shifts the current calendar date by a set number of months.
 * Updates the global `activeGrid.fullDateArray` state.
 * @param {number} direction - The number of months to move (positive for future, negative for past).
 */
const step = (direction) => {
    let [year, month, day] = activeGrid.fullDateArray;

    const dateObj = new Date(year, month - 1, 1);
    dateObj.setMonth(dateObj.getMonth() + direction);

    year = dateObj.getFullYear();
    month = dateObj.getMonth() + 1;

    activeGrid.fullDateArray = [year, month, day];
};

/**
 * Starts the continuous stepping logic when a user holds down a button.
 * Prevents default event behavior and initializes the interval.
 * @param {Event} e - The triggering UI event (Mouse, Touch, or Keyboard).
 * @param {number} direction - Direction of the step (1 or -1).
 */
const startHolding = (e, direction) => {
    e.preventDefault();
    
    step(direction);
    
    holdTimer = setInterval(() => step(direction), 150);
};

/**
 * Stops the continuous stepping logic and clears the active timer.
 */
const stopHolding = () => {
    clearInterval(holdTimer);
    holdTimer = null;
};

/**
 * Attaches mouse, touch, and keyboard listeners to an element to handle navigation.
 * Supports single clicks, long-presses, and accessibility (Enter/Space keys).
 * @param {HTMLElement} el - The button/element to attach events to.
 * @param {number} direction - The direction to step when activated.
 */
const attachHoldEvents = (el, direction) => {
    const start = (e) => startHolding(e, direction);
    
    el.addEventListener('mousedown', start);
    el.addEventListener('touchstart', start, {passive: false});
    
    ['mouseup', 'mouseleave', 'touchend', 'touchcancel'].forEach(evt => {
        el.addEventListener(evt, stopHolding);
    });

    el.addEventListener('keydown', (e) => {
        if ((e.key === 'Enter' || e.key === ' ') && !e.repeat) {
            e.preventDefault();
            start(e);
        }
    });

    el.addEventListener('keyup', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            stopHolding(e);
        }
    });
};

/**
 * Main initialization function for the date selector.
 * Finds navigation buttons in the DOM and binds their logic.
 * @example
 * setupDateSelect();
 */
const setupDateSelect = () => {
    const prevBtn = document.getElementById('prevMonthCalendar');
    const nextBtn = document.getElementById('nextMonthCalendar');

    if (prevBtn) attachHoldEvents(prevBtn, -1);
    if (nextBtn) attachHoldEvents(nextBtn, 1);
};

export { changeCalendarDateSelectorDate };
export default setupDateSelect;