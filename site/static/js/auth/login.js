import InitOauth from "./oauthBtn.js";
import InitForm  from "./form.js";
import InitKeepNext from "./keep-next.js";

const urlParams = new URLSearchParams(window.location.search);
const nextUri = urlParams.get('next') || "/";

InitOauth(nextUri);
InitKeepNext(nextUri);
InitForm(() => window.location.href = nextUri);
