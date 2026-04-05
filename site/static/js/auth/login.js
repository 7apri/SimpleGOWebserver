import setupForm  from "./components/form.js";
import InitKeepNext from "./components/keep-next.js";

const urlParams = new URLSearchParams(window.location.search);
let nextUrl = urlParams.get('next');

if(nextUrl === null){
    nextUrl = '/'
} else {
    InitKeepNext(nextUrl);
}

const form = document.getElementById("form");
setupForm(form,null,async (r) => {
    const data = await r.json();
    switch (data.status){
        case "pending":
            let next = "/2fa"
            if (nextUrl != '/'){
                next += `?next=${nextUrl}`
            }
            window.location.href = next;
            break;
        default:
            window.location.href = nextUrl;
    }
});
