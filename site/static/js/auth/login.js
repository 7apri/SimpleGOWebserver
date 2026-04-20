import setupForm  from "./components/form.js";
import InitKeepNext from "./components/keep-next.js";

const urlParams = new URLSearchParams(window.location.search);
let nextUrl = urlParams.get('next');
let nextUrlEncoded = encodeURIComponent(nextUrl);

if(nextUrl === null){
    nextUrl = '/';
} else {
    InitKeepNext(nextUrlEncoded);
}

const form = document.getElementById("form");
const errDsp = document.getElementById("err-dsp");

const onRespOK = async (r) => {
    const data = await r.json();
    switch (data.status){
        case "pending":
            let next = "/2fa"
            if (nextUrl != '/'){
                next += `?next=${nextUrlEncoded}`
            }
            window.location.href = next;
            break;
        default:
            window.location.href = nextUrl;
    }
};

const providers = new Map(
  Array.from(document.querySelectorAll('[data-provider]'), el => [
    el.dataset.provider, 
    el
  ])
);
const onRespNotOk = async (r) => {
    const contentType = r.headers.get("content-type");
    if (!contentType || !contentType.startsWith("application/json")) {
        errDsp.textContent = await r.text();
        return;
    }
    const data = await r.json();
    switch(data.code){
        case "use_oauth":
            data.data.allowed.forEach( provider => {
                providers.get(provider).classList.add("suggested");
            });
            break;
    }
    errDsp.textContent = data.error;
};


setupForm(form,onRespOK,onRespNotOk);