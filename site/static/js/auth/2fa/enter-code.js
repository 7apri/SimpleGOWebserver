import setupForm from "../components/form.js";
import setupCodeInput from "../components/code-input.js";

const setupMFa = (codeInput, boxes, codeFrom, recoveryCodeForm, stateToggle, errDsp,onSuccess) =>{
    setupCodeInput(codeInput, boxes);
    let isCodeEnterState = false;
    
    const switchState = () =>{
        isCodeEnterState = !isCodeEnterState;
        
        if( isCodeEnterState ){
            codeFrom.classList.remove("hidden");
            recoveryCodeForm.classList.add("hidden");
            stateToggle.textContent = tr("switch_recovery");
        } else {
            recoveryCodeForm.classList.remove("hidden");
            codeFrom.classList.add("hidden");
            stateToggle.textContent = tr("switch_code");
        }
    }
    switchState();
    
    stateToggle.addEventListener("click", switchState);

    const onRespNotOk = async (r) => {
        const contentType = r.headers.get("content-type");
        if (!contentType || !contentType.startsWith("application/json")) {
            errDsp.textContent = await r.text();
            return;
        }
        const data = await r.json();
        errDsp.textContent = data.error;
    };

    setupForm(codeFrom, onSuccess, onRespNotOk);
    setupForm(recoveryCodeForm, onSuccess, onRespNotOk);
    
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
}
export default setupMFa;