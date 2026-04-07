import setupDateInput from "./dateInput.js";
import setupTaskInput from "./taskInput.js";
import setupTaskList from "./taskList.js";

/**
 * Handles the inicial setup of the todo widget.
 * 
 */
const main = () =>{
    setupDateInput();
    setupTaskInput();
    setupTaskList();
};

export default main;