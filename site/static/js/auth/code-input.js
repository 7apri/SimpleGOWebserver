const InitCodeInput = () =>{
    const input = document.getElementById('code-input');
    const boxes = document.getElementsByClassName('auth-code-box');

    input.addEventListener('input', (e) => {
        const val = e.target.value.replace(/[^0-9]/g, '');
        e.target.value = val;
        
        Array.from(boxes).forEach((box, i) => {
            const char = val[i];
            
            box.textContent = char || '';
            
            box.classList.toggle('active', i === val.length && document.activeElement === input);
        });
    });

    input.addEventListener('focus', () => input.dispatchEvent(new Event('input')));
    input.addEventListener('blur', () => {
        Array.from(boxes).forEach(b => b.classList.remove('active'));
    });

    window.addEventListener('DOMContentLoaded', () => {
        input.focus();
    });
}
export default InitCodeInput
