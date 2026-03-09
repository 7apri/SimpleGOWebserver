/**
 * Controller for the task input form.
 * Handles user input submission and coordination between the input field and task creation logic.
 * @module TaskInputController
 */

import { addTask } from "./task.js";

/**
 * Initializes the task input listeners.
 * Listens for form submission to capture and process new task entries.
 * @function setup
 */
const setup = () => {
    /** @type {HTMLInputElement|null} Date input reference for associating tasks with dates */
    const dateInputEl = document.getElementById("dateInputTodo");
    
    /** @type {HTMLInputElement|null} Text input for the task description */
    const taskInputEl = document.getElementById("addInput");

    /**
     * Extracts values from the DOM, validates content, and triggers the task addition.
     * Defaults to the current date if no date is selected.
     * @private
     */
    const addTaskFromInput = () => {
        // Prevent adding empty or whitespace-only tasks
        if (taskInputEl.value.trim() === "") return;

        const dateValue = dateInputEl.value || new Date().toISOString().split('T')[0];
        const taskContent = taskInputEl.value;

        addTask(dateValue, taskContent);

        // Reset the input field for the next task
        taskInputEl.value = "";
    };

    /**
     * Event listener for the parent form's submit event.
     * Supports both clicking the submit button and pressing 'Enter'.
     */
    taskInputEl.parentElement.addEventListener('submit', (e) => {
        e.preventDefault();
        addTaskFromInput();
    });
};

export default setup;