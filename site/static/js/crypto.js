const CRYPTO_DB = 'AppCryptoDatabase';
const KEY_STORE = 'identity_keys';

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
            tx.onabort = tx.onerror = () => resolve(null);
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
    isRegistering: false,

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
        this.isRegistering = true;

        try {
            const deviceId = await this.initDeviceCookie();
            let keyPair = await cryptoDB.get('identity_keypair'); 
            
            // Phase 1: Ensure keys exist locally
            if (!keyPair) {
                console.log("generating new cryptographic device identity...");
                keyPair = await window.crypto.subtle.generateKey(
                    { name: "ECDH", namedCurve: "P-256" },
                    true,
                    ["deriveKey", "deriveBits"]
                );
                await cryptoDB.set('identity_keypair', keyPair);
            }

            await this.uploadPublicKey(deviceId, keyPair.publicKey);

        } catch (err) {
            console.error("Crypto verification/generation failed:", err);
        } finally {
            this.isRegistering = false;
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

            if (!response.ok) {
                const errorText = await response.text();
                console.error("SERVER REJECTED KEY UPLOAD:", response.status, errorText);
                return;
            }

            if (response.ok) {
                console.log("cryptographic identity securely locked to server session.");
            }
        } catch (err) {
            console.error("network error:", err);
        }
    },
    async generateKeyPayloadForUsers(roomKey, userIds) {
        const keyResponse = await fetch('/api/crypto/fetch-keys', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ user_ids: userIds })
        });
        const targetDevices = await keyResponse.json();
        const ourKeyPair = await cryptoDB.get('identity_keypair');
        const rawRoomKeyBytes = await window.crypto.subtle.exportKey("raw", roomKey);
        const encryptedKeysPayload = [];

        for (const device of targetDevices) {
            try {
                const devicePublicBytes = Uint8Array.from(atob(device.public_key), c => c.charCodeAt(0));
                const devicePublicKey = await window.crypto.subtle.importKey(
                    "raw", devicePublicBytes, { name: "ECDH", namedCurve: "P-256" }, true, []
                );
                const sharedSecretKey = await window.crypto.subtle.deriveKey(
                    { name: "ECDH", public: devicePublicKey }, ourKeyPair.privateKey, { name: "AES-GCM", length: 256 }, true, ["encrypt"]
                );
                const iv = window.crypto.getRandomValues(new Uint8Array(12)); 
                const encryptedKeyBuffer = await window.crypto.subtle.encrypt(
                    { name: "AES-GCM", iv: iv }, sharedSecretKey, rawRoomKeyBytes
                );
                const combined = new Uint8Array(iv.length + encryptedKeyBuffer.byteLength);
                combined.set(iv);
                combined.set(new Uint8Array(encryptedKeyBuffer), iv.length);
                
                encryptedKeysPayload.push({
                    device_id: device.device_id,
                    encrypted_room_key: btoa(String.fromCharCode(...combined))
                });
            } catch (err) { }
        }
        return encryptedKeysPayload;
    },
    async startDirectMessage(targetUserId) {
        try {
            const initRes = await fetch(`/api/dm/init/${targetUserId}`, { method: 'POST' });
            const { room_id, is_new } = await initRes.json();

            if (is_new) {
                const roomKey = await window.crypto.subtle.generateKey(
                    { name: "AES-GCM", length: 256 }, true, ["encrypt", "decrypt"]
                );
                
                const encryptedKeysPayload = await this.generateKeyPayloadForUsers(roomKey, [targetUserId]);

                await fetch(`/api/rooms/${room_id}/keys`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ encrypted_keys: encryptedKeysPayload })
                });

                await cryptoDB.set(`room_key_${room_id}`, roomKey);

            } else {
                await this.ensureRoomKeyCached(room_id); 
            }

            htmx.ajax('GET', `/chats/${room_id}`, { target: 'body' });

        } catch (err) {
            console.error("Failed to initialize DM:", err);
            alert("Failed to establish secure connection.");
        }
    },
   async createEncryptedRoom(roomType, roomName, participantUserIds) {
    const roomKey = await window.crypto.subtle.generateKey(
        { name: "AES-GCM", length: 256 },
        true,
        ["encrypt", "decrypt"]
    );

        const encryptedKeysPayload = await this.generateKeyPayloadForUsers(roomKey, participantUserIds);

        const result = await fetch('/api/rooms', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                type: roomType,
                name: roomName,
                participants: participantUserIds,
                encrypted_keys: encryptedKeysPayload
            })
        });

        if (result.ok) {
            const parsedUrl = new URL(result.headers.get("HX-Redirect"), window.location.origin);
            const newRoomId = parsedUrl.pathname.split("/").pop();
            await cryptoDB.set(`room_key_${newRoomId}`, roomKey);
            
            htmx.ajax('GET', parsedUrl.pathname, { target: 'body' });
        }
    },
    async ensureRoomKeyCached(roomId) {
        const cachedKey = await cryptoDB.get(`room_key_${roomId}`);
        if (cachedKey) return true;

        const response = await fetch(`/api/crypto/room-key/${roomId}`);
        if (!response.ok) return false;
        
        const data = await response.json();
        if (!data.encrypted_key) return false;

        const ourKeyPair = await cryptoDB.get('identity_keypair');
        const serverDevicePublicBytes = Uint8Array.from(atob(data.sender_public_key), c => c.charCodeAt(0));
        
        const senderPublicKey = await window.crypto.subtle.importKey(
            "raw", serverDevicePublicBytes, { name: "ECDH", namedCurve: "P-256" }, true, []
        );

        const sharedSecretKey = await window.crypto.subtle.deriveKey(
            { name: "ECDH", public: senderPublicKey },
            ourKeyPair.privateKey,
            { name: "AES-GCM", length: 256 },
            true,
            ["decrypt"]
        );

        const combinedBytes = Uint8Array.from(atob(data.encrypted_key), c => c.charCodeAt(0));
        const iv = combinedBytes.slice(0, 12);
        const ciphertext = combinedBytes.slice(12);

        const decryptedRawBuffer = await window.crypto.subtle.decrypt(
            { name: "AES-GCM", iv: iv },
            sharedSecretKey,
            ciphertext
        );

        const roomCryptoKey = await window.crypto.subtle.importKey(
            "raw", decryptedRawBuffer, { name: "AES-GCM", length: 256 }, true, ["encrypt", "decrypt"]
        );

        await cryptoDB.set(`room_key_${roomId}`, roomCryptoKey);
        return true;
    }
};
window.CryptoEngine.sendMessage = async function(roomId, plainText) {
    const roomKey = await cryptoDB.get(`room_key_${roomId}`);
    if (!roomKey) throw new Error("Room key not found. Cannot encrypt message.");

    const encoder = new TextEncoder();
    const data = encoder.encode(plainText);
    const iv = window.crypto.getRandomValues(new Uint8Array(12));

    const encryptedBuffer = await window.crypto.subtle.encrypt(
        { name: "AES-GCM", iv: iv },
        roomKey,
        data
    );

    const encryptedBase64 = btoa(String.fromCharCode(...new Uint8Array(encryptedBuffer)));
    const ivBase64 = btoa(String.fromCharCode(...iv));

    const formData = new FormData();
    formData.append("content_encrypted", encryptedBase64);
    formData.append("nonce", ivBase64);

    return htmx.ajax('POST', `/api/rooms/${roomId}/messages`, {
        values: {
            content_encrypted: encryptedBase64,
            nonce: ivBase64
        },
        target: '#chat-messages',
        swap: 'beforeend'
    });
};

class EncryptedMessage extends HTMLElement {
    async connectedCallback() {
        const encryptedContent = this.getAttribute('content');
        const nonceBase64 = this.getAttribute('nonce');
        const roomId = this.getAttribute('room-id');

        if (!encryptedContent || !nonceBase64 || !roomId) {
            this.renderError("malformed message frame");
            return;
        }

        try {
            const nvm = await window.CryptoEngine.ensureRoomKeyCached(roomId);
            console.log(nvm);
            const roomKey = await cryptoDB.get(`room_key_${roomId}`);
            if (!roomKey) {
                this.renderError("message encrypted (Key missing)");
                return;
            }

            const encryptedBytes = Uint8Array.from(atob(encryptedContent), c => c.charCodeAt(0));
            const nonceBytes = Uint8Array.from(atob(nonceBase64), c => c.charCodeAt(0));

            const decryptedBuffer = await window.crypto.subtle.decrypt(
                {
                    name: "AES-GCM",
                    iv: nonceBytes
                },
                roomKey,
                encryptedBytes
            );

            const plainText = new TextDecoder().decode(decryptedBuffer);

            this.textContent = plainText;
        } catch (err) {
            console.error("decryption failure:", err);
            this.renderError("decryption failed (Corrupted data)");
        }
    }

    renderError(msg) {
        this.innerHTML = `<span style="color: var(--error-color, #ff4444); font-style: italic; font-size: 0.9em;">${msg}</span>`;
    }
}
customElements.define('encrypted-message', EncryptedMessage);

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => window.CryptoEngine.verifyAndRegisterIdentity());
} else {
    window.CryptoEngine.verifyAndRegisterIdentity();
}

document.body.addEventListener('htmx:afterOnLoad', () => {
    window.CryptoEngine.verifyAndRegisterIdentity();
});