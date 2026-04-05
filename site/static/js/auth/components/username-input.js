import { SetLoadingEl, ResetLoadEl } from "../../util/loadingEffect.js";
import GetCsrfToken from "./csrf.js";

function Init () {
    const userNameInput = document.getElementById("username-input");
    if (!userNameInput) return;
    const userNameStatus = document.getElementById("username-status");

    const userSuggestionsWrapper = document.getElementById("username-suggestions");
    const userSuggestions = userSuggestionsWrapper.querySelectorAll("ul li button");

    const suggestionsClickEvent = (suggestion) => () => {
        userNameInput.value = suggestion.textContent;
        userNameInput.focus(); 
        userNameInput.dispatchEvent(new Event('input', { bubbles: true }));
    };
        
    for(let i = 0;  i < userSuggestions.length; i++){
        userSuggestions[i].addEventListener("click", suggestionsClickEvent(userSuggestions[i]));
    }

    const userNameStatusIcon = userNameStatus.querySelector("use");
    const iconBaseUrl = userNameStatusIcon.getAttribute('href');

    let debounceTimer = null;

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

            if (response.ok) {
                setStatus('success');
            } else {
                setStatus('error');
            }
            if (response.status === 409) {
                const data = await response.json();
                userSuggestions.forEach((btn, i) => {
                    const suggestion = data.data[i];

                    if (!suggestion) {
                        btn.parentElement.style.display = "none";
                        return; 
                    }

                    btn.textContent = suggestion;
                    btn.parentElement.style.display = "";
                });
            }
        } catch (err) {
            console.error("Error checking username:", err);
        }
    };

    const eventFunc = () => {
        const username = userNameInput.value.trim()
        clearTimeout(debounceTimer);

        if (username.length > 0) {
            setStatus('loading'); 
            debounceTimer = setTimeout(() => verifyUsername(username), 500);
        } else {
            setStatus('idle');
        }
    }

    userNameInput.addEventListener("input", eventFunc);
    document.addEventListener("DOMContentLoaded", eventFunc);

    function setStatus(state) {
        userNameStatus.style.display = (state === 'idle') ? 'none' : '';
        if(state === 'idle' || state === 'success') {
            userSuggestionsWrapper.classList.add("hidden");
        }
        
        ResetLoadEl(userNameStatus);
        
        let isValid = false;

        if (state === 'success') {
            isValid = true;
            userNameStatusIcon.setAttribute('href', `${iconBaseUrl}#check`);
            userNameInput.setAttribute('aria-invalid', 'false');
        }

        else if (state === 'loading') {
            SetLoadingEl(userNameStatus);
        } 

        else if (state === 'error') {
            userNameStatusIcon.setAttribute('href', `${iconBaseUrl}#cross`);
            userSuggestionsWrapper.classList.remove("hidden");
        }
        
        userNameInput.setAttribute('aria-invalid', !isValid);
        if(isValid){
            userNameInput.setCustomValidity("");
        }else{
            userNameInput.setCustomValidity(tr("username_invalid"));
        }
    }
}
Init();