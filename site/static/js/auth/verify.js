import setUpForm from "./components/form.js";
import InitKeepNext from "./components/keep-next.js";
import GetCsrfToken from "./components/csrf.js";
import { ResetLoadEl,SetLoadingEl } from "../util/loadingEffect.js";

const emailForm   = document.getElementById("form-email");
const codeForm    = document.getElementById("form-code");
const confirmForm = document.getElementById("form-confirm");

const backBtn = document.getElementById("state-back");
const statesWrapper = document.getElementById('states-wrapper');

if( statesWrapper.dataset.loggedIn === "false" && statesWrapper.dataset.state === "success"){
    window.location.href = `/2fa?next=${encodeURIComponent(window.location.href + "?") + window.location.search}`
}

function setState(newState) {
    statesWrapper.querySelectorAll('[data-state]').forEach(el => {
        el.classList.toggle('hidden', el.dataset.state !== newState);
    });

    if (newState === 'email' || newState === 'success') {
        backBtn.classList.add('hidden');
    } else {
        backBtn.classList.remove('hidden');
    }
    
    statesWrapper.dataset.state = newState;
}

backBtn.addEventListener('click', () => {
    if (statesWrapper.dataset.state  === 'code') {
        setState('email');
    } 
});

const urlParams = new URLSearchParams(window.location.search);
let nextUrl = urlParams.get('next');

if(nextUrl === null){
    nextUrl = '/'
} else {
    InitKeepNext(nextUrl);
}

setUpForm(emailForm, null,  () => setState("code") );
setUpForm(codeForm,null,async (r) => {
    if (!r.headers.get("Content-Type") === "application/json") {
        setState("success");
        return;
    }
    const data = await r.json()
    switch (data.status){
        case "pending":
            window.location.href = `/2fa?next=${encodeURIComponent(window.location.href + "?") + window.location.search}`;
            break;
        default:
            setState("success");
    }
});

setUpForm(confirmForm, null,   () => window.location.href = nextUrl);



let cooldownActive = false;
const resendBtn = document.getElementById("resend-btn");

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

if (resendBtn.dataset.endpoint){
    resendBtn.addEventListener('click', async (e) =>{
        if (cooldownActive) return;
    
        SetLoadingEl(resendBtn);
    
        try {
            let token = await GetCsrfToken();
            const resp = await fetch(resendBtn.dataset.endpoint, {
                method: 'POST',
                headers: { 
                    'Content-Type' : 'application/json',
                    'Accept'       : 'application/json',
                    'X-CSRF-Token': token,
                },
            });
            if (!resp.ok) {
                const respJSON = await resp.json();
                if (resp.status == 429){
                    respJSON.data && startCooldown(respJSON.data.retry_after)
                }
            }
        } catch (err) {
            console.log(err);
        }
        ResetLoadEl(resendBtn);
    });
} else{
    resendBtn.disabled = true;
}