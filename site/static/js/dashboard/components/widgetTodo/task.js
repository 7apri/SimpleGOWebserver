/**
 * Persistence layer for tasks.
 * Manages CRUD operations in LocalStorage and synchronizes state with the Calendar UI.
 * @module TaskService
 */

import { activeGrid } from "../widgetCalendar/grid.js";
import { createTask } from "./taskList.js";

/** * Key used for storing tasks in LocalStorage.
 * @constant {string} 
 */
const STORAGE_KEY = 'tasks';

/**
 * Retrieves all tasks from LocalStorage.
 * @returns {Object.<string, Array<{id: number, content: string, completed: boolean}>>} 
 * An object where keys are date strings (YYYY-MM-DD) and values are arrays of task objects.
 */
const getAllTasks = () => JSON.parse(localStorage.getItem(STORAGE_KEY)) || {};

/**
 * Adds a new task to a specific date and saves it to storage.
 * Updates the Calendar grid indicators upon success.
 * @param {string} dateStr - The date for the task (YYYY-MM-DD).
 * @param {string} taskContent - The text description of the task.
 * @returns {Object} The newly created task object.
 */
const addTask = (dateStr, taskContent) => {
    const all = getAllTasks();
    
    if (!all[dateStr]) all[dateStr] = [];

    const newTask = {
        id: Date.now(),
        content: taskContent,
        completed: false
    };

    // UI: Add task to the current task list view
    createTask(newTask, dateStr);
    
    // Data: Save to the head of the array
    all[dateStr].unshift(newTask);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(all));
    
    // Sync: Tell the calendar to update the visual "dot" for this day
    activeGrid.updateSelectedDay();
    
    return newTask;
};

/**
 * Retrieves the list of tasks for a specific date.
 * @param {string} dateStr - Date string (YYYY-MM-DD).
 * @returns {Array} Array of task objects or an empty array if none found.
 */
const getTasksByDate = (dateStr) => {
    const all = getAllTasks();
    return all[dateStr] || [];
};

/**
 * Toggles the completion status of a task.
 * @param {string} dateStr - The date associated with the task.
 * @param {number} taskId - The unique ID of the task.
 */
const toggleTask = (dateStr, taskId) => {
    const all = getAllTasks();
    if (!all[dateStr]) return;

    all[dateStr] = all[dateStr].map(task => {
        return task.id === taskId ? { ...task, completed: !task.completed } : task;
    });

    localStorage.setItem(STORAGE_KEY, JSON.stringify(all));

    // Sync: Update calendar grid as completion might change the "completed" visual style
    activeGrid.updateSelectedDay();
};

/**
 * Deletes a task from a specific date. If no tasks remain for that date, 
 * the date key is removed from storage.
 * @param {string} dateStr - The date associated with the task.
 * @param {number} taskId - The unique ID of the task.
 */
const removeTask = (dateStr, taskId) => {
    const all = getAllTasks();
    if (!all[dateStr]) return;
    
    all[dateStr] = all[dateStr].filter(task => task.id !== taskId);
    
    // Clean up empty date keys to keep storage tidy
    if (all[dateStr].length === 0) {
        delete all[dateStr];
    }
    
    localStorage.setItem(STORAGE_KEY, JSON.stringify(all));
    
    // Sync: Update calendar grid to potentially remove the "hasTasks" dot
    activeGrid.updateSelectedDay();
};

export { getAllTasks, addTask, getTasksByDate, toggleTask, removeTask };