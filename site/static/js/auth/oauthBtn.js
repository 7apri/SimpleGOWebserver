const ResetLoadEl = (el) => {
    el.disabled = false;
    el.classList.remove('loading');
}
const SetLoadingEl = (el) => {
    el.disabled = true;
    el.classList.add('loading');
}

const Init = (nextUri) =>{
    window.addEventListener('pageshow', (event) => {
        if (event.persisted) {
            document.querySelectorAll('.loading').forEach(ResetLoadEl);
        }
    });
    
    document.querySelectorAll('.oauth-btn').forEach( btn => {
        btn.onclick = async () => {
            const provider = btn.getAttribute("data-provider");
            const target = `/api/auth/e/login?provider=${provider}&next=${encodeURIComponent(nextUri)}`;
    
            SetLoadingEl(btn);
            window.location.href = target;
    
            setTimeout(() => { ResetLoadEl(btn) }, 5000);
        };
    });
}

export {ResetLoadEl, SetLoadingEl};
export default Init;
