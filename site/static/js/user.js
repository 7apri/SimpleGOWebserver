import { RegisterPath } from "./router.js";

const enter = () =>{
    document.getElementById("back-btn").addEventListener("click", () => window.history.back());
    
};
RegisterPath("/*", {
    enter: enter,
    exit: () => console.log("exit user")
})