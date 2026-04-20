import { ResetLoadEl, SetLoadingEl } from "../../util/loadingEffect.js";
const Init = (nextUri) =>{
    document.querySelectorAll('[data-provider]').forEach( btn => {
        btn.onclick = async (e) => {
            e.preventDefault();
            const provider = btn.getAttribute("data-provider");
            const target = `/api/auth/e/login?provider=${provider}&next=${encodeURIComponent(nextUri)}`;
    
            SetLoadingEl(btn);
            window.location.href = target;
    
            setTimeout(() => { ResetLoadEl(btn) }, 5000);
        };
    });
}
const urlParams = new URLSearchParams(window.location.search);
const nextUrl = urlParams.get('next') || '/';
Init(nextUrl);
