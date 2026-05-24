import InitForm from "./components/form.js";
const urlParams = new URLSearchParams(window.location.search);
let nextUrl = urlParams.get('next');

if(nextUrl === null){
    nextUrl = '/'
}

const form = document.getElementById("form");
InitForm(form,null,async (r) => {
    if (!r.headers.get("Content-Type") === "application/json") {
        window.location.href = nextUrl
        return;
    }
    const data = await r.json()
    switch (data.status){
        case "pending":
            let next = "/2fa"
            if (nextUrl != '/'){
                next += `?next=${nextUrlEncoded}`
            }
            window.location.href = next;
            break;
        default:
            window.location.href = next;
    }
});