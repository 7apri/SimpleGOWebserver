import InitForm from "./auth/components/form.js";
const urlParams = new URLSearchParams(window.location.search);
let nextUrl = urlParams.get('next');

if(nextUrl === null){
    nextUrl = '/'
}
InitForm(async (resp) => console.log(resp));