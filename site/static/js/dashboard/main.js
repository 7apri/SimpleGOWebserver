/**
 * Application Entry Point.
 * Orchestrates the initialization of all specialized widgets and theming.
 * @module Main
 */

import setupTheme from "../util/theme.js";
import setUpWeather from "./components/widgetWeather/component.js";
import setUpTodo from "./components/widgetTodo/component.js";
import setUpCalendar from "./components/widgetCalendar/component.js";

/**
 * Sets up theme preferences (light/dark mode) from local storage or system settings.
 */
setupTheme();

/**
 * Initialize Widgets.
 * Each widget handles its own internal DOM binding and state logic.
 */
setUpWeather();
setUpTodo();
setUpCalendar();

/** @type {HTMLElement|null} Button to toggle global shortcuts and settings */
const allOpen = document.getElementById('open-all');

/**
 * Global UI Controller for the Navigation/Settings menu.
 * Toggles the visibility and accessibility states of global dashboard panels.
 */
if (allOpen) {
  allOpen.addEventListener('click', (e) => {
    const isExpanded = e.currentTarget.getAttribute('aria-expanded') === 'true';
    
    // Update Accessibility State
    e.currentTarget.setAttribute('aria-expanded', !isExpanded);
    
    // Toggle UI Panels
    document.getElementById('shortcuts').classList.toggle('open');
    document.getElementById('settings').classList.toggle('open');
  });
}