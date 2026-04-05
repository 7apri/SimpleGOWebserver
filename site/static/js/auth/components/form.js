import { ResetLoadEl, SetLoadingEl } from "../../util/loadingEffect.js";
import GetCsrfToken from "./csrf.js";

/**
 * @param {HTMLElement} form
 * @param {HTMLElement} errorDsp
 * @param {function(Response): void} onRespOK
 */
const setupForm = (form, errorDsp ,onRespOK = null) =>{
    form.addEventListener("submit", async e => {
        e.preventDefault();

        const submitBtn = e.submitter || form.querySelector('button[type="submit"]');
        if(!submitBtn) return;

        SetLoadingEl(submitBtn);
        if(errorDsp != null){
            errorDsp.textContent = '';
        }

        const formData = new FormData(form);
        const endpoint = submitBtn.dataset.endpoint || form.dataset.endpoint;
        if(endpoint == undefined){
            console.error("No defined endpoint on form");
            return;
        }
        const data = Object.fromEntries(formData.entries());
        
        try {
            const token = await GetCsrfToken();
            const resp = await fetch(endpoint, {
                method: 'POST',
                headers: {
                    'X-CSRF-Token': token,
                    'Content-Type': 'application/json',
                    'Accept': 'application/json'
                },
                body: JSON.stringify(data),
            });
            if (resp.ok) {
                onRespOK?.(resp);
                ResetLoadEl(submitBtn);
            } else {
                const data = await resp.json();
                if(errorDsp != null){
                    errorDsp.textContent = data.error;
                } 

                if (resp.status === 403) {
                    GetCsrfToken(true);
                }
                
                ResetLoadEl(submitBtn);
            }
        } catch (err) {
            console.error(`Form error ${err}`);
            ResetLoadEl(submitBtn);
        }
    });
}

export default setupForm;