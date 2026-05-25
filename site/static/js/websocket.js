const webSocket = {
    endpoint : (window.location.protocol === 'http:' ? 'ws' : 'wss') + `://${window.location.host}/ws-reload`,
    socket   : null,
    connect  : () => {
        self.socket = new WebSocket(endpoint);

        socket.onmessage = function(event) {
            const signal = event.data;

            if (signal === 'r') {
                window.location.reload();
            } 
            else if (signal === 'c') {
                const assets = document.querySelectorAll('link[rel="stylesheet"]:not([data-no-hot]), img:not([data-no-hot])');
                assets.forEach(el => {
                    const attr = el.tagName === 'LINK' ? 'href' : 'src';
                    
                    const url = new URL(el[attr], window.location.href);
                    url.searchParams.set('hot', Date.now());
                    el[attr] = url.href;
                });
            }
        };

        socket.onerror = (error) => {
            console.error("webSocket Error:", error);
        };

        socket.onclose = (event) => {
            if (event.wasClean) {
                if (event.code === 1001 || event.code === 1006) {
                    setTimeout(connect, 3000);
                }
            } else {
                setTimeout(connect, 2000);
            }
        };
    }
}