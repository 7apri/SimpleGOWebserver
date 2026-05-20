let cachedToken = null;

async function GetCsrfToken(forceRefresh = false) {
    if (cachedToken && !forceRefresh){
        return cachedToken;
    } 

    try {
        let resp = await fetch('/api/csrf', {
            headers: { 'Accept': 'application/json' }
        });
        if (!resp.ok) {
            throw new Error('CSRF fetch failed')
        }
        
        const data = await resp.json();
        cachedToken = data.token;
        return cachedToken;
    } catch (err) {
        console.error("CSRF Error:", err);
        return "";
    }
}

export default GetCsrfToken;