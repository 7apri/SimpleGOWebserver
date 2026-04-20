import setUpForm from "./components/form.js";
import setupCodeInput from "./components/code-input.js";
import InitKeepNext from "./components/keep-next.js";
import setupMFa from "./2fa/enter-code.js"
import GetCsrfToken from "./components/csrf.js";
import { ResetLoadEl,SetLoadingEl } from "../util/loadingEffect.js";

const emailForm   = document.getElementById("form-email");
const codeForm    = document.getElementById("form-code");
const confirmForm = document.getElementById("form-confirm");

const backBtn = document.getElementById("state-back");
const statesWrapper = document.getElementById('states-wrapper');

const codeInput = document.getElementById("code-input");
const boxes = document.getElementById("code").children;
setupCodeInput(codeInput, boxes);

const codeInput2FA =  document.getElementById("code-input-2fa");
const stateToggle2FA = document.getElementById("2fa-state-toggle");
if( codeInput2FA ){
    const form2FA = document.getElementById("form-2fa");
    const form2FARecovery = document.getElementById("recovery-code-form");
    const boxes2FA = document.getElementById("code-2fa").children;

    setupMFa(codeInput2FA,boxes2FA,form2FA, form2FARecovery, stateToggle2FA, errDsp, () => setState("success"))
}

if( statesWrapper.dataset.loggedIn === "false" && statesWrapper.dataset.state === "success"){
    window.location.href = `/2fa?next=${encodeURIComponent(window.location.href + "?") + window.location.search}`
}

function setState(newState) {
    statesWrapper.querySelectorAll('[data-state]').forEach(el => {
        el.classList.toggle('hidden', el.dataset.state !== newState);
    });

    if (newState === 'email' || newState === 'success') {
        console.log("hello");
        backBtn.classList.add('hidden');
    } else {
        backBtn.classList.remove('hidden');
    }
    
    statesWrapper.dataset.state = newState;

    if( !stateToggle2FA ) return;
    if (newState === '2fa'){
        stateToggle2FA.classList.remove('hidden');
    }else{
        stateToggle2FA.classList.add('hidden');
    }
}

backBtn.addEventListener('click', () => {
    if (statesWrapper.dataset.state  === 'code') {
        setState('email');
    }
    if (statesWrapper.dataset.state  === '2fa') {
        setState('code');
    } 
});

const urlParams = new URLSearchParams(window.location.search);
let nextUrl = urlParams.get('next');

if(nextUrl === null){
    nextUrl = '/'
} else {
    InitKeepNext(nextUrl);
}

const errDsp   = document.getElementById("err-dsp");
const onRespNotOk = async (r) => {
    const contentType = r.headers.get("content-type");
    if (!contentType || !contentType.startsWith("application/json")) {
        errDsp.textContent = await r.text();
        return;
    }
    const data = await r.json();
    errDsp.textContent = data.error;
};

setUpForm(emailForm,() => setState("code"), onRespNotOk);
setUpForm(codeForm,async (r) => {
    const contentType = r.headers.get("Content-Type")
    if (contentType == null || !(contentType === "application/json")) {
        setState("success");
        return;
    }
    const data = await r.json()
    switch (data.status){
        case "pending":
            setState("2fa");
            break;
        default:
            setState("success");
    }
}, onRespNotOk);

setUpForm(confirmForm,() => window.location.href = nextUrl, onRespNotOk);

let cooldownActive = false;
const resendBtn = document.getElementById("resend-btn");

function startCooldown(seconds) {
    cooldownActive = true;
    resendBtn.disabled = true;
    resendBtn.style.opacity = "0.5";
    resendBtn.style.cursor = "not-allowed";

    let remaining = seconds;

    const upd = () => {
        if (remaining <= 0) {
            clearInterval(interval);
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
            console.error(err);
        }
        ResetLoadEl(resendBtn);
    });
} else{
    resendBtn.disabled = true;
}