import GetCsrfToken from "./components/csrf.js";

const showStep = (stepId) => {
    ['step-init', 'step-scan', 'step-recovery'].forEach(id => {
        document.getElementById(id).classList.toggle('hidden', id !== stepId);
    });
};

document.getElementById('btn-start-2fa').addEventListener('click', async () => {
    const token = await GetCsrfToken();
    const res = await fetch('/api/auth/2fa/init',{
        method: 'POST',
        headers: {
            'X-CSRF-Token': token,
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        },
    });
    if (res.ok) {
        const data = await res.json();
        document.getElementById('qr-image').src = `data:image/png;base64,${data.qr_code}`;
        document.getElementById('manual-secret').textContent = data.secret;
        showStep('step-scan');
    }
});


document.getElementById('btn-verify-2fa').addEventListener('click', async () => {
    const token = await GetCsrfToken();
    const code = document.getElementById('verify-code').value;
    const res = await fetch('/api/auth/2fa/enable', {
        method: 'POST',
        headers: {
            'X-CSRF-Token': token,
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        },
        body: JSON.stringify({ code }),
    });
    
    if (res.ok) {
        const data = await res.json();
        const list = document.getElementById('recovery-list');
        list.innerHTML = ''; 
        
        data.recovery_codes.forEach(code => {
            const el = document.createElement('div');
            el.className = 'recovery-item';
            el.textContent = code;
            list.appendChild(el);
        });
        document.getElementById('btn-download-codes').addEventListener('click', () => {
            const content = "PANELS RECOVERY CODES\n" + 
                            "Keep these in a safe place. Each code can be used once.\n\n" + 
                            data.recovery_codes.join('\n');
        
            const blob = new Blob([content], { type: 'text/plain' });
        
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            
            a.href = url;
            a.download = `recovery-codes-${new Date().toISOString().slice(0,10)}.txt`;
            
            document.body.appendChild(a);
            a.click();
            window.URL.revokeObjectURL(url);
            document.body.removeChild(a);
        });
        
        showStep('step-recovery');
    } else {
        alert("Invalid code. Please try again.");
    }
});

document.getElementById('btn-finish-2fa').addEventListener('click', () => {
    location.reload();
});