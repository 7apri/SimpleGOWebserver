import { RegisterPath } from "./router.js";
RegisterPath("/*", {
    enter: () => console.log("enter user"),
    exit: () => console.log("exit user")
})