/**
* Sets the state
* @param {HTMLElement} target
*/
const SetLoadingEl = (target) =>{
    target.classList.add('loading');
    target.disabled = true;
};
/**
* Resets the state
* @param {HTMLElement} target
*/
const ResetLoadEl = (target) =>{
    target.classList.remove('loading');
    target.disabled = false;
};
export { ResetLoadEl, SetLoadingEl };