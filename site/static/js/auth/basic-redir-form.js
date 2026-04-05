import InitForm from "./components/form.js";
const urlParams = new URLSearchParams(window.location.search);
let nextUrl = urlParams.get('next');

if(nextUrl === null){
    nextUrl = '/'
}
const form = document.getElementById("form");
InitForm(form,null,async () => window.location.href = nextUrl);