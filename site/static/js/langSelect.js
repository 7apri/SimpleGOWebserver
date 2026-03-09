const langElements = document.getElementsByClassName("lang");
const currentLang = document.documentElement.getAttribute('lang');

const langMenu = document.getElementById('lang-menu');
const langBtn = document.getElementById('langSwitch-btn');

langBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    langMenu.classList.toggle('open');
});

document.addEventListener('click', (e) => {
    if (!langMenu.contains(e.target) && langMenu.classList.contains('open')) {
        langMenu.classList.remove('open');
    }
});

for (const el of langElements){
    const lang = el.getAttribute("data-lang");
    const btnEl = el.querySelector("button");

    if( lang === currentLang ){ 
        btnEl.classList.add('bg-primary'); 
    }

    btnEl.addEventListener('click', async () => {
        if( currentLang === lang ) return;
        
        try {
            document.cookie = `lang=${lang};path=/; SameSite=Lax`;
            window.location.reload();
        } catch (err) {
            console.error("Failed to switch language:", err);
        }
    });
};
