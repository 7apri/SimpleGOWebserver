/**
 * Renders the entire daily list into the DOM element thats provided.
 * Or by default into the element with the id daily-list.
 * !! DOESNT NOT DO ANYTHING YET !!
 * 
 * @param {Array<Object>} dailyArray - The array of daily data to render from.
 * @param {HTMLUListElement} element - The element to render into.
 * 
 * @returns {boolean} Either true or false depens on how it went.
 */
const renderDailyList = (dailyArray, element = document.getElementById("daily-list")) =>  {
    try{
        return true;
    }
    catch(err){
        console.error("Got an error rendering the day: "+ err +" on element: " + element);
        return false;
    }
}

export default renderDailyList;
