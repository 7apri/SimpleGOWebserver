/**
 * Module for managing the Todo date input widget.
 * Handles manual date selection, day-by-day navigation, 
 * and synchronized list rendering.
 * @module TodoDateInput
 */

import { activeGrid } from "../widgetCalendar/grid.js";
import { renderList } from "./taskList.js";

/** * Promise that resolves when the DOM is fully loaded.
 * Used to ensure elements are available before event binding.
 * @type {Promise<void>} 
 */
const domReady = new Promise(resolve => {
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", resolve);
    } else {
        resolve();
    }
});

/** @type {HTMLInputElement|null} Reference to the date input field */
const dateInputEl = document.getElementById('dateInputTodo');

/**
 * Updates the date input value and triggers a re-render of the task list.
 * @param {string} dateString - The date in YYYY-MM-DD format.
 */
const changeDateInputContent = (dateString) => {
    dateInputEl.value = dateString;
    renderList(dateInputEl.value);
};

/**
 * Initializes the Todo date controls, handles acceleration logic for long-presses,
 * and sets up synchronization with the Calendar grid.
 * @async
 * @function setup
 */
const setup = async () => {

    /** @type {ReturnType<typeof setInterval>|null} Timer for long-press intervals */
    let holdTimer = null;
    
    /** * Current speed of date skipping. Increases while holding a button.
     * @type {number} 
     */
    let velocity = 1;

    /**
     * Increments or decrements the current date based on velocity.
     * Updates both the Todo input and the global Calendar grid.
     * @param {number} direction - Direction of movement (-1 for past, 1 for future).
     */
    const step = (direction) => {
        const date = new Date(dateInputEl.value);
        
        // Calculate the number of days to jump based on velocity
        const jump = Math.trunc(velocity) * direction;
        date.setDate(date.getDate() + jump);
        
        const newDateStr = date.toISOString().split('T')[0];
        changeDateInputContent(newDateStr);
        
        // Sync the main calendar grid
        activeGrid.fullDateString = dateInputEl.value;

        // Exponential acceleration up to a cap of 30 days per interval
        if (velocity < 30) velocity *= 1.1;
    };

    /**
     * Starts the stepping process and resets velocity.
     * @param {Event} e - The triggering event.
     * @param {number} direction - -1 for previous day, 1 for next day.
     */
    const startHolding = (e, direction) => {
        e.preventDefault();
        velocity = 1;
        
        step(direction); 
        
        holdTimer = setInterval(() => step(direction), 100);
    };

    /**
     * Stops the stepping process and resets velocity to baseline.
     */
    const stopHolding = () => {
        clearInterval(holdTimer);
        holdTimer = null;
        velocity = 1;
    };

    /**
     * Binds mouse, touch, and keyboard events to a navigation element.
     * @param {HTMLElement} el - The button element.
     * @param {number} direction - The direction to step.
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

    // Attach listeners to UI buttons
    attachHoldEvents(document.getElementById('prevDayTodo'), -1);
    attachHoldEvents(document.getElementById('nextDayTodo'), 1);

    await domReady;

    /**
     * Event listener for manual input changes.
     * Ensures the list and calendar stay in sync when a user picks a date via the native picker.
     */
    dateInputEl.addEventListener('change', (e) => {
        renderList(e.currentTarget.value);
        activeGrid.fullDateString = dateInputEl.value;
    });

    // Initial state: Set to current date on load
    const todayStr = new Date().toISOString().split('T')[0];
    changeDateInputContent(todayStr);
    activeGrid.fullDateString = dateInputEl.value;
}

export { changeDateInputContent };
export default setup;