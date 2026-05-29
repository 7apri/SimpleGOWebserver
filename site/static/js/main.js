import { RegisterPath } from "./router.js";
document.getElementById('side-left-info-more-btn').addEventListener("click", () => document.getElementById('side-left-info-more').classList.toggle('hide'))
document.addEventListener('htmx:configRequest', (event) => {
    if (window.location.pathname === event.detail.path) {
        event.preventDefault();
    }
});

const enter = () =>{
    console.log("enter home");
};
RegisterPath("/", {
    enter: enter,
    exit: () => console.log("exit home")
})
