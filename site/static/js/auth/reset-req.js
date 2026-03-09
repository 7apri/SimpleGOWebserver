import InitLang  from "./langSelect.js";
import InitForm  from "./form.js";
import InitKeepNext from "./keep-next.js";

const urlParams = new URLSearchParams(window.location.search);
const nextUri = urlParams.get('next') || "/";

InitKeepNext(nextUri);
InitForm(() => window.location.reload());
InitLang();
