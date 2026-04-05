const input = document.getElementById('code-input');
const boxes = document.querySelectorAll('.auth-code-box');

const updateUI = () => {
    const value = input.value.replace(/\D/g, '');;
    input.value = value;
    const curPos = input.selectionStart;

    let wasBeforeActive = false;

    boxes.forEach((box, i) => {
        box.textContent = value[i] || '';

        const isActive = (i === curPos -1) || (curPos === boxes.length && i === boxes.length - 1);
        
        if((wasBeforeActive || i == 0 && curPos == 0) && document.activeElement === input){
            box.classList.add('waiting');
        }else{
            box.classList.remove('waiting');
        }
        if (isActive && document.activeElement === input) {
            box.classList.add('active');
            wasBeforeActive = true;
        } else {
            box.classList.remove('active');
            wasBeforeActive = false;
        }
    });
};

['input', 'click', 'keyup', 'focus', 'blur'].forEach(event => {
    input.addEventListener(event, updateUI);
});

updateUI();