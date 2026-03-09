import InitLang from "./langSelect.js";
import InitForm from "./form.js";
import InitKeepNext from "./keep-next.js";
import InitCodeInput from "./code-input.js";

const url = new URL(window.location);
const nextUri = url.searchParams.get('next') || "/";

InitKeepNext(nextUri);
InitForm(() => {
    window.location.href = nextUri;
});
InitLang();
