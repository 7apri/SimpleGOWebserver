import setupForm from "./components/form.js";
import GetCsrfToken from "./components/csrf.js";
import InitKeepNext from "./components/keep-next.js";
import { ResetLoadEl,SetLoadingEl } from "../util/loadingEffect.js";

const url = new URL(window.location);
const nextUri = url.searchParams.get('next') || "/";

if(nextUri === null){
    nextUri = '/'
} else {
    InitKeepNext(nextUri);
}

const form = document.getElementById("form");
setupForm(form,null,() => window.location.reload());

const errorDsp = document.getElementById('error-dsp');
const errorDsp2 = document.getElementById('error-dsp2');
const resendBtn = document.getElementById('resend-btn');
const resendEnd = resendBtn.dataset.endpoint;

let cooldownActive = false;

function startCooldown(seconds) {
    cooldownActive = true;
    resendBtn.disabled = true;
    resendBtn.style.opacity = "0.5";
    resendBtn.style.cursor = "not-allowed";

    let remaining = seconds;

    const upd = () => {
        errorDsp2.textContent = `${remaining}s`;

        if (remaining <= 0) {
            clearInterval(interval);
            errorDsp.textContent = "";
            errorDsp2.textContent = "";
            resendBtn.disabled = false;
            resendBtn.style.opacity = "1";
            resendBtn.style.cursor = "pointer";
            cooldownActive = false;
        }
    }
    upd()

    const interval = setInterval(() => {
        remaining--;
        upd()
    }, 1000);
}

if (resendEnd){
    resendBtn.addEventListener('click', async (e) =>{
        if (cooldownActive) return;
    
        SetLoadingEl(resendBtn);
        errorDsp.textContent = "";
    
        try {
            let token = await GetCsrfToken();
            const resp = await fetch(resendEnd, {
                method: 'POST',
                headers: { 
                    'Content-Type' : 'application/json',
                    'Accept'       : 'application/json',
                    'X-CSRF-Token': token,
                },
            });
            if (!resp.ok) {
                const respJSON = await resp.json();
                errorDsp.textContent = respJSON.error;
                if (resp.status == 429){
                    respJSON.data && startCooldown(respJSON.data.retry_after)
                }
            }
        } catch (err) {
            errorDsp.textContent = tr("err_network");
        }
        ResetLoadEl(resendBtn);
    });
} else{
    resendBtn.disabled = true;
}

