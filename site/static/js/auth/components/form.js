import { ResetLoadEl, SetLoadingEl } from "../../util/loadingEffect.js";
import GetCsrfToken from "./csrf.js";

/**
 * @param {HTMLElement} form
 * @param {function(Response): void} onRespOK
 * @param {function(Response): void} onRespNotOK
 */
const setupForm = (form ,onRespOK = null, onRespNotOK = null) =>{
    var checkboxes = form.querySelectorAll("input[type='checkbox']");
    form.addEventListener("submit", async e => {
        e.preventDefault();

        const submitBtn = e.submitter || form.querySelector('button[type="submit"]');
        if(!submitBtn) return;

        SetLoadingEl(submitBtn);

        const formData = new FormData(form);
        const endpoint = submitBtn.dataset.endpoint || form.dataset.endpoint;
        if(endpoint == undefined){
            console.error("No defined endpoint on form");
            return;
        }
        const data = Object.fromEntries(formData.entries());
        checkboxes.forEach( checkbox =>{
            data[checkbox.getAttribute("name") || "undefined"] = checkbox.checked;
        })
        
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
            } else {
                onRespNotOK?.(resp);
            }
        } catch (err) {
            console.error(`Form error ${err}`);
        }
        ResetLoadEl(submitBtn);
    });
}

export default setupForm;