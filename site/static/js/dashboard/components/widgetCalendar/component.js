import setupDateSelect from "./dateSelector.js";
import setupGrid from "./grid.js";

/**
 * Handles the inicial setup of the calendar widget.
 * 
 */
const main = () =>{
    setupDateSelect();
    setupGrid();
};

export default main;