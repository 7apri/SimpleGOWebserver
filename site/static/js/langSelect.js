const langElements = document.getElementsByClassName("lang");
const currentLang = document.documentElement.getAttribute('lang');

const langMenu = document.getElementById('lang-menu');
const langBtn = document.getElementById('langSwitch-btn');

let isOpen = false

if (langBtn) {
    document.addEventListener('click', (e) => {
        if (!langMenu.contains(e.target) && langMenu.classList.contains('open')) {
            langMenu.classList.remove('open');
        }
    });
    langBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        if( isOpen ){
            langMenu.classList.remove('open');
            isOpen = false;
            return
        }
        langMenu.classList.add('open');
        langMenu.classList.add('t');
        isOpen = true;
    });
}

langMenu.addEventListener('transitionend', () =>{
    !isOpen && langMenu.classList.remove('t');
})

for (const el of langElements){
    const lang = el.getAttribute("data-lang");
    const btnEl = el.querySelector("button");

    if( lang === currentLang ){ 
        btnEl.classList.add('bg-primary'); 
    }

    btnEl.addEventListener('click', async () => {
        if( currentLang === lang ) return;
        
        try {
            const exp = new Date;
            exp.setFullYear(exp.getFullYear() + 1);
            document.cookie = `lang=${lang};expires=${exp.toUTCString()};path=/;SameSite=Lax;Secure=true`;
            window.location.reload();
        } catch (err) {
            console.error("failed to switch language:", err);
        }
    });
};
