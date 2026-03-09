import InitLang from "./langSelect.js";
import InitForm from "./form.js";
import InitKeepNext from "./keep-next.js";
import InitCodeInput from "./code-input.js";
import { ResetLoadEl,SetLoadingEl } from "./oauthBtn.js";

const url = new URL(window.location);
const nextUri = url.searchParams.get('next') || "/";

InitKeepNext(nextUri);
InitCodeInput();
InitForm((_,data) => {
    const code = data.get('code');
        if(code == "" ) return;

    url.searchParams.set("c", code);
    window.history.replaceState({}, '', url);
    window.location.reload()
});

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

resendBtn.addEventListener('click', async (e) =>{
    if (cooldownActive) return;

    SetLoadingEl(resendBtn);
    errorDsp.textContent = "";

    try {
        const resp = await fetch(resendEnd, {
            method: 'POST',
            headers: { 
                'Content-Type' : 'application/json',
                'Accept'       : 'application/json',
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

InitLang();

