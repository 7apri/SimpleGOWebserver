import { RegisterPath } from "./router.js";
document.getElementById('side-left-info-more-btn')?.addEventListener("click", () => document.getElementById('side-left-info-more').classList.toggle('hide'))
document.addEventListener('htmx:configRequest', (event) => {
    if (window.location.pathname === event.detail.path) {
        event.preventDefault();
    }
});
const postInputResize = (e) =>{
    e.target.style.height = 'auto';
    e.target.style.height = e.target.scrollHeight + 'px';
}
const enterHome = () =>{
    document.querySelector(".post-input")?.addEventListener("input", postInputResize)
};
RegisterPath("/", {
    enter: enterHome,
    exit: () => console.log("exit home")
})
