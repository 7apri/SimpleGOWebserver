import InitOauth from "./oauthBtn.js";
import InitLang  from "./langSelect.js";
import InitForm  from "./form.js";
import InitPasswordInput from "./password-input.js";
import InitKeepNext from "./keep-next.js";

const urlParams = new URLSearchParams(window.location.search);
const nextUri = urlParams.get('next') || "/";

InitOauth(nextUri);
InitKeepNext(nextUri);
InitPasswordInput();
InitForm(() => window.location.href = `/account-verify` + (nextUri ===  "/" ? '' : `?next=${nextUri}`));
InitLang();
