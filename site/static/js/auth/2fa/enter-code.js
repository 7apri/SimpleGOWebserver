import setupForm from "../components/form.js";

const urlParams = new URLSearchParams(window.location.search);
let nextUrl = urlParams.get('next');

if(nextUrl === null){
    nextUrl = '/'
} else {
    InitKeepNext(nextUrl);
}

const codeFrom = document.getElementById("digit-code-form");
const recoveryCodeFrom = document.getElementById("recovery-code-form");
const stateToggle = document.getElementById("state-toggle");

let isCodeEnterState = false;

const switchState = () =>{
    isCodeEnterState = !isCodeEnterState;
    
    if( isCodeEnterState ){
        codeFrom.classList.remove("hidden");
        recoveryCodeFrom.classList.add("hidden");
        stateToggle.textContent = tr("switch_recovery");
    } else {
        recoveryCodeFrom.classList.remove("hidden");
        codeFrom.classList.add("hidden");
        stateToggle.textContent = tr("switch_code");
    }
}
switchState();

stateToggle.addEventListener("click", switchState);

setupForm(codeFrom,null, () => window.location.href = nextUrl);
setupForm(recoveryCodeFrom,null, () => window.location.href = nextUrl);

const recoveryInput = document.getElementById('recovery-code-input');

recoveryInput.addEventListener('input', (e) => {
    const start = e.target.selectionStart;
    let value = e.target.value
        .toLowerCase()
        .replace(/\s+/g, '-') 
        .replace(/--+/g, '-')
        .replace(/[^a-z0-9-]/g, '');
    let words = value.split('-');
    if (words.length > 3) {
        words = words.slice(0, 3);
        value = words.join('-');
    }

    if (words.length < 3) {
        recoveryInput.setCustomValidity(tr("recovery_code_length"));
    } else {
        recoveryInput.setCustomValidity("");
    }

    e.target.value = value;
    e.target.setSelectionRange(start, start);

});