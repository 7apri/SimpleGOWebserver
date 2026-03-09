import { ResetLoadEl, SetLoadingEl } from "./oauthBtn.js";

const InitForm = (onRespOK) =>{
    const formElement = document.getElementById('form');
    const errorElement = document.getElementById('error-dsp');

    if (formElement) {
        const submitBtn = formElement.querySelector('button[type="submit"]');
        const endpoint = formElement.dataset.endpoint;

        formElement.addEventListener('submit', async e => {
            e.preventDefault();

            SetLoadingEl(submitBtn);
            errorElement.textContent = '';

            const formData = new FormData(e.target)
            const data = Object.fromEntries(formData.entries());
            
            try {
                const resp = await fetch(endpoint, {
                    method: 'POST',
                    headers: { 
                        'Content-Type': 'application/json',
                        'Accept' : 'application/json',
                     },
                    body: JSON.stringify(data),
                });
                if (resp.ok) {
                    onRespOK?.(resp, formData);
                } else {
                    const data = await resp.json()
                    errorElement.textContent = data.error;
                    
                    ResetLoadEl(submitBtn);
                }
            } catch (err) {
                errorElement.textContent = tr("err_network");
                ResetLoadEl(submitBtn);
            }
        });
    }
}
export default InitForm;
