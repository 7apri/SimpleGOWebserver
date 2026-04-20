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
const errDsp = document.getElementById("err-dsp");

const onRespOK = () => window.location.href = `/account-verify` + (nextUrl ===  "/" ? '' : `?next=${encodeURIComponent(nextUrl)}`);
const onRespNotOk = async (r) => {
    const contentType = r.headers.get("content-type");
    if (!contentType || !contentType.startsWith("application/json")) {
        errDsp.textContent = await r.text();
        return;
    }
    const data = await r.json();
    errDsp.textContent = data.error;
};

setupForm(form,onRespOK,onRespNotOk);