const passwordInput = document.querySelector("#password-input");
const passwordToggle = document.getElementById('password-toggle');
if (passwordToggle && passwordInput) {
    const passwordIcon = passwordToggle.querySelector('use');
    
    const update = (isPasswordVisible) =>{
        passwordToggle.setAttribute('aria-label', isPasswordVisible ? tr('hide_password') : tr('show_password'));    
        passwordIcon.setAttribute('href', isPasswordVisible ? '/static/assets/iconBundle.svg#eye-slash' : '/static/assets/iconBundle.svg#eye');
    };
    update(passwordInput.type === 'text');

    passwordToggle.addEventListener("click", () =>{
        const isPasswordVisible = passwordInput.type === 'text';
        passwordInput.type = isPasswordVisible ?  'password' : 'text';
        update(!isPasswordVisible);
    });
}
