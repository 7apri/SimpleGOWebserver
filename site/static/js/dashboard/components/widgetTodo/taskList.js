/**
 * Module responsible for rendering and managing the task list UI.
 * Handles task creation, deletion animations, and checkbox interactions.
 * @module TaskListView
 */

import { removeTask, toggleTask, getAllTasks } from "./task.js";

/** @type {HTMLElement|null} The container element where tasks are rendered */
const taskList = document.getElementById('taskList');

/**
 * Initializes the task list by setting up event delegation for task actions
 * and performing the initial render based on the current date.
 * @function setup
 */
const setup = () => {
    /** @type {HTMLInputElement|null} Reference to the date input to identify which day's tasks to modify */
    const dateInput = document.getElementById('dateInputTodo');

    /**
     * Event Delegation: Handles clicks on checkboxes and delete buttons within the list.
     */
    taskList.addEventListener('click', (e) => {
        const taskItem = e.target.closest('.task');
        if (!taskItem) return;

        const taskId = Number(taskItem.dataset.id);

        // --- Handle Task Removal ---
        if (e.target.closest('.remove-task-btn')) {
            /**
             * Helper to find the next focusable neighbor while skipping items being animated.
             * @param {HTMLElement} item - The current task element.
             * @param {'prev'|'next'} direction - The direction to look.
             */
            const getValidNeighbor = (item, direction) => {
                let neighbor = direction === 'prev' ? item.previousElementSibling : item.nextElementSibling;
                while (neighbor && neighbor.classList.contains('removing')) {
                    neighbor = direction === 'prev' ? neighbor.previousElementSibling : neighbor.nextElementSibling;
                }
                return neighbor;
            };

            const prevTask = getValidNeighbor(taskItem, 'prev');
            const nextTask = getValidNeighbor(taskItem, 'next');

            // Accessibility: Move focus to a neighbor or back to the input before removing the element
            if (prevTask) {
                prevTask.querySelector('.remove-task-btn')?.focus();
            } else if (nextTask) {
                nextTask.querySelector('.remove-task-btn')?.focus();
            } else {
                document.getElementById('addInput').focus();
            }

            // Trigger CSS animation and data removal
            taskItem.classList.add('removing');
            removeTask(dateInput.value, taskId);

            taskItem.addEventListener('transitionend', () => {
                taskItem.remove();
            }, { once: true });
        }

        // --- Handle Task Toggle (Complete/Incomplete) ---
        if (e.target.classList.contains('taskCheck')) {
            taskItem.classList.toggle('completed');
            toggleTask(dateInput.value, taskId);
        }
    });

    // Perform initial load
    renderList(dateInput.value);
}

/**
 * Re-renders the entire task list for a specific date.
 * Groups tasks by completion status (active on top, completed below).
 * @param {string} dateStr - The date string (YYYY-MM-DD).
 */
const renderList = (dateStr) => {
    const tasks = (getAllTasks())[dateStr];

    // Index 0: active tasks, Index 1: completed tasks
    let finalHtml = ["", ""];

    if (tasks) {
        tasks.forEach(task => {
            const isCompleted = task.completed;
            const labelId = "task-" + task.id;
            
            finalHtml[isCompleted ? 1 : 0] += `
                <li class="task flex jc-space-between ali-center ${isCompleted ? 'completed' : ''}" data-id="${task.id}">
                    <div class="flex ali-center">
                        <input type="checkbox" id="${labelId}" class="taskCheck border-highlight" ${isCompleted ? 'checked' : ''}/>
                        <label class="taskContent" for="${labelId}">${task.content}</label>
                    </div>
                    <button class="remove-task-btn icon-container border-highlight" aria-label="Odtranit úkol ${task.content}" type="button">
                        <svg class="icon" aria-hidden="true">
                            <use href="./assets/iconBundle.svg#cross"></use>
                        </svg>
                    </button>
                </li>`;
        });
    }

    taskList.innerHTML = finalHtml.join("");
}

/**
 * Dynamically adds a single task element to the top of the list.
 * Used for immediate feedback when a user adds a new task.
 * @param {Object} taskObj - The task data object.
 * @param {string} dateStr - The date associated with the task.
 */
const createTask = (taskObj, dateStr) => {
    const taskEl = document.createElement('li');
    taskEl.className = `task flex jc-space-between ali-center just-added ${taskObj.completed ? 'completed' : ''}`;
    taskEl.dataset.id = taskObj.id;
    taskEl.dataset.date = dateStr;

    const labelId = "task-" + taskObj.id;
    taskEl.innerHTML = `
        <div class="flex ali-center">
            <input type="checkbox" id="${labelId}" class="taskCheck border-highlight" ${taskObj.completed ? 'checked' : ''}/>
            <label class="taskContent" for="${labelId}">${taskObj.content}</label>
        </div>
        <button class="remove-task-btn icon-container border-highlight" aria-label='Odtranit úkol "${taskObj.content}"' type="button">
             <svg class="icon" aria-hidden="true">
                <use href="./assets/iconBundle.svg#cross"></use>
            </svg>
        </button>`;

    taskList.prepend(taskEl);
};

export { renderList, createTask };
export default setup;