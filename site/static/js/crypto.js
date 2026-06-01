const CRYPTO_DB = 'AppCryptoDatabase';
const KEY_STORE = 'identity_keys';

// Global cache to prevent constant connection spawning
let cachedDBInstance = null;

const cryptoDB = {
    open() {
        if (cachedDBInstance) return Promise.resolve(cachedDBInstance);
        return new Promise((resolve, reject) => {
            const request = indexedDB.open(CRYPTO_DB, 1);
            request.onupgradeneeded = () => request.result.createObjectStore(KEY_STORE);
            request.onsuccess = () => {
                cachedDBInstance = request.result;
                resolve(cachedDBInstance);
            };
            request.onerror = () => reject(request.error);
        });
    },
    async get(key) {
        const db = await this.open();
        return new Promise((resolve) => {
            const tx = db.transaction(KEY_STORE, 'readonly');
            const req = tx.objectStore(KEY_STORE).get(key);
            req.onsuccess = () => resolve(req.result);
            tx.onabort = tx.onerror = () => resolve(null); // Fail safely
        });
    },
    async set(key, val) {
        const db = await this.open();
        return new Promise((resolve, reject) => {
            const tx = db.transaction(KEY_STORE, 'readwrite');
            tx.objectStore(KEY_STORE).put(val, key);
            tx.oncomplete = () => resolve();
            tx.onerror = () => reject(tx.error);
        });
    }
};

window.CryptoEngine = {
    isRegistering: false, // Prevents duplicate network requests fighting each other

    async initDeviceCookie() {
        let deviceId = await cryptoDB.get('device_id');
        if (!deviceId) {
            const match = document.cookie.match(/(^|;)\s*device_id\s*=\s*([^;]+)/);
            if (match) {
                deviceId = match[2];
                await cryptoDB.set('device_id', deviceId);
            }
        }
        if (!deviceId) {
            deviceId = crypto.randomUUID();
            await cryptoDB.set('device_id', deviceId);
        }
        document.cookie = `device_id=${deviceId}; path=/; max-age=31536000; SameSite=Lax; Secure`;
        return deviceId;
    },

    async verifyAndRegisterIdentity() {
        if (this.isRegistering) return;
        
        const deviceId = await this.initDeviceCookie();
        
        // HARSH FIX: Don't trust the client-side access_token cookie.
        // Instead, check for the device_id server state. If the user is logged out,
        // the registration fetch will fail gracefully with a 401 anyway.
        let keyPair = await cryptoDB.get('identity_keypair');
        
        if (!keyPair) {
            this.isRegistering = true;
            try {
                console.log("🔒 Generating new cryptographic device identity...");
                keyPair = await window.crypto.subtle.generateKey(
                    { name: "ECDH", namedCurve: "P-256" },
                    true,
                    ["deriveKey", "deriveBits"]
                );
                
                await cryptoDB.set('identity_keypair', keyPair);
                await this.uploadPublicKey(deviceId, keyPair.publicKey);
            } catch (err) {
                console.error("Crypto generation failed:", err);
            } finally {
                this.isRegistering = false;
            }
        }
    },

    async uploadPublicKey(deviceId, publicKeyObj) {
        const exportedPublic = await window.crypto.subtle.exportKey("raw", publicKeyObj);
        const base64PublicKey = btoa(String.fromCharCode(...new Uint8Array(exportedPublic)));

        try {
            const response = await fetch('/api/crypto/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    device_id: deviceId,
                    public_key: base64PublicKey
                })
            });

            if (response.ok) {
                console.log("✅ Cryptographic identity securely locked to server session.");
            } else if (response.status === 401 || response.status === 403) {
                // User is not logged in or token expired. 
                // Wipe the un-uploaded keypair so it regenerates properly when they actually log in.
                await cryptoDB.set('identity_keypair', null);
            }
        } catch (err) {
            console.error("🚨 Network error while registering crypto identity:", err);
        }
    }
};

// Handle initial page load (Matches Go's initial raw template execution on "/")
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => window.CryptoEngine.verifyAndRegisterIdentity());
} else {
    window.CryptoEngine.verifyAndRegisterIdentity();
}

// Intercept HTMX mutations smoothly
document.body.addEventListener('htmx:afterOnLoad', () => {
    window.CryptoEngine.verifyAndRegisterIdentity();
});