var webSocket = {
    endpoint : (window.location.protocol === 'http:' ? 'ws' : 'wss') + `://${window.location.host}/api/ws`,
    socket   : null,
    connect  : function() {
        this.socket = new WebSocket(this.endpoint);

        this.socket.onmessage = (event) => {
            const data = JSON.parse(event.data);
            console.log(data);
        };

        this.socket.onerror = (error) => {
            console.error("webSocket Error:", error);
        };

        this.socket.onclose = (event) => {
            if (event.wasClean && event.code === 1001) {
                setTimeout(() => this.connect(), 3000);
            } else {
                setTimeout(() => this.connect(), 2000);
            }
        };
    }
}
webSocket.connect();
            /*
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
            */