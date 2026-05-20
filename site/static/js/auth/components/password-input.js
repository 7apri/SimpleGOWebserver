const passwordInput = document.getElementById("password-input");
const passwordToggle = document.getElementById('password-toggle');

if (passwordToggle && passwordInput) {
    const passwordIcon = passwordToggle.querySelector('use');
    
    if (passwordIcon) {
        const iconBaseUrl = passwordIcon.getAttribute('href').split('#')[0];

        const update = (isVisible) => {
            passwordInput.type = isVisible ? 'text' : 'password';
            
            const label = isVisible ? tr('hide_password') : tr('show_password');
            passwordToggle.setAttribute('aria-label', label);
            
            passwordIcon.setAttribute('href', `${iconBaseUrl}#${isVisible ? 'eye-slash' : 'eye'}`);
        };

        passwordToggle.addEventListener("click", () => {
            const willBeVisible = passwordInput.type === 'password';
            update(willBeVisible);
        });
        
        update(passwordInput.type === 'text');
    }
}