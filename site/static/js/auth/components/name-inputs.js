import { SetLoadingEl, ResetLoadEl } from "../../util/loadingEffect.js";
import GetCsrfToken from "./csrf.js";

const displayNameInput = document.getElementById("display-name-input");
const displayNameLen = document.getElementById("display-name-lenght");

const userNameStatus = document.getElementById("username-status");
const userNameInput = document.getElementById("username-input");
const hintDisplayUsername = document.getElementById("username-hint");
const hintTextUsername = hintDisplayUsername.textContent;

const userNameStatusIcon = userNameStatus.querySelector("use");
const iconBaseUrl = userNameStatusIcon.getAttribute('href');
const maxUsernameLen = parseInt(userNameInput.getAttribute('maxlength'), 10);
const invalidUsernameMsg = userNameInput.dataset.errorMsg || "Invalid username"; 

const userSuggestionsWrapper = document.getElementById("username-suggestions");
const userSuggestions = userSuggestionsWrapper.querySelectorAll("ul li button");

let isUsernameUserEdited = userNameInput.value.length > 0;
let debounceTimer = null;

const slugify = (str) => {
    return str.normalize("NFD")             
        .replace(/[\u0300-\u036f]/g, "") 
        .toLowerCase()
        .trim()
        .replace(/\s+/g, '-')
        .replace(/[^a-z0-9-]/g, '')
        .replace(/-+/g, '-')
        .replace(/^-+|-+$/g, '');
};

displayNameInput.addEventListener('input', (e) => {
    const val = e.target.value;
    displayNameLen.textContent = val.length;
    
    if (!isUsernameUserEdited && val.length <= maxUsernameLen) {
        userNameInput.value = slugify(val);
        startDebounce(userNameInput.value);
    }
});

userNameInput.addEventListener('input', (e) => {
    const input = e.target;
    isUsernameUserEdited = input.value.length > 0;

    const oldVal = input.value;
    const start = input.selectionStart;

    const value = oldVal
        .toLowerCase()
        .replace(/\s+/g, '-') 
        .replace(/--+/g, '-')
        .replace(/[^a-z0-9-]/g, '')
        .replace(/^-/g, '');
    
    if (oldVal !== value) {
        input.value = value;
        const lenDiff = oldVal.length - value.length;
        const newPos = Math.max(0, start - lenDiff);
        input.setSelectionRange(newPos, newPos);
    }

    startDebounce(value);
});

const setStatus = (state) => {
    userSuggestionsWrapper.classList.toggle("hidden", state !== "suggest");
    userNameStatus.style.display = (state === 'idle') ? 'none' : '';

    if (state === "loading") {
        SetLoadingEl(userNameStatus);
    } else {
        ResetLoadEl(userNameStatus);
    }

    const config = {
        success: { icon: "check", invalid: "false", validity: "" },
        suggest: { icon: "cross", invalid: "true",  validity: invalidUsernameMsg },
        error:   { icon: "cross", invalid: "true",  validity: invalidUsernameMsg },
        loading: { icon: "",      invalid: "true",  validity: "" }
    };

    const current = config[state];
    if (current) {
        if (current.icon) userNameStatusIcon.setAttribute('href', `${iconBaseUrl}#${current.icon}`);
        userNameInput.setAttribute('aria-invalid', current.invalid);
        userNameInput.setCustomValidity(current.validity);
    }
};
const verifyUsername = async (username) => {
    try {
        const token = await GetCsrfToken();
        const response = await fetch('/api/verify-username', {
            method: 'POST',
            headers: {
                'X-CSRF-Token': token,
                'Content-Type': 'application/json',
                'Accept': 'application/json'
            },
            body: JSON.stringify({ Username: username }),
        });

        if (response.ok) return setStatus('success');
        
        const data = await response.json();
        if (response.status === 409) {
            userSuggestions.forEach((btn, i) => {
                const suggestion = data.data[i];
                if (!suggestion) {
                    btn.parentElement.style.display = "none";
                    return; 
                }
                btn.textContent = suggestion;
                btn.parentElement.style.display = "";
            });
            return setStatus('suggest');
        }
        
        hintDisplayUsername.textContent = data.error;
        hintDisplayUsername.classList.add("error");
        return setStatus('error');
    } catch (err) {
        console.error("Error checking username:", err);
        setStatus('error');
    }
};

const startDebounce = (u) => {
    clearTimeout(debounceTimer);
    if (u.length >= 3) {
        setStatus('loading'); 
        debounceTimer = setTimeout(() => verifyUsername(u), 500);
    } else {
        setStatus('idle');
    }
};

userSuggestionsWrapper.addEventListener("click", (e) => {
    const btn = e.target.closest('button');
    if (!btn) return;
    
    userNameInput.value = btn.textContent;
    userNameInput.focus();
    userNameInput.dispatchEvent(new Event('input', { bubbles: true })); 
});

if (isUsernameUserEdited) {
    startDebounce(userNameInput.value);
}