import { RegisterPath } from "./router.js";

const enter = () =>{
    document.getElementById("back-btn").addEventListener("click", () => window.history.back());
    const more = document.getElementById("user-profile-more");
    if(more){
        document.getElementById("user-profile-more-btn")?.addEventListener("click", () => more.classList.toggle("hide"));
    }
};
RegisterPath("/*", {
    enter: enter,
    exit: () => console.log("exit user")
})