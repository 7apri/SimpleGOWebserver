const setupCodeInput = (input, boxes) => {
    const updateUI = () => {
        const value = input.value.replace(/\D/g, '');;
        input.value = value;
        const curPos = input.selectionStart;
    
        let wasBeforeActive = false;
    
        for( let i = 0; i < boxes.length; i++){
            boxes[i].textContent = value[i] || '';
    
            const isActive = (i === curPos -1) || (curPos === boxes.length && i === boxes.length - 1);
            
            if((wasBeforeActive || i == 0 && curPos == 0) && document.activeElement === input){
                boxes[i].classList.add('waiting');
            }else{
                boxes[i].classList.remove('waiting');
            }
            if (isActive && document.activeElement === input) {
                boxes[i].classList.add('active');12
                wasBeforeActive = true;
            } else {
                boxes[i].classList.remove('active');
                wasBeforeActive = false;
            }
        }
    };
    
    ['input', 'click', 'keyup', 'focus', 'blur'].forEach(event => {
        input.addEventListener(event, updateUI);
    });
    
    updateUI();
}
export default setupCodeInput;