import { RegisterPath } from "./router.js";

const backBtnEvent = () => window.history.back();
const profileMoreEvent = (element) => () => element.classList.toggle("hide");

const enter = () =>{
    document.getElementById("back-btn").addEventListener("click", backBtnEvent);
    const more = document.getElementById("user-profile-more");
    if(more) {
        document.getElementById("user-profile-more-btn")?.addEventListener("click", profileMoreEvent(more));
    }
};
RegisterPath("/*", {
    enter: enter,
    exit: () => console.log("exit user")
})