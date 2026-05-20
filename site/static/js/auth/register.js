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

setupForm(form,null,() => window.location.href = `/account-verify` + (nextUrl ===  "/" ? '' : `?next=${encodeURIComponent(nextUrl)}`));
