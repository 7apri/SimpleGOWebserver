const wsAction = Object.freeze({
    APPEND:  'a',
    PREPEND: 'p',
    REPLACE: 'r'
});
var WS = {
    endpoint : (window.location.protocol === 'http:' ? 'ws' : 'wss') + `://${window.location.host}/api/ws`,
    socket   : null,
    connect  : function() {
        this.socket = new WebSocket(this.endpoint);

        this.socket.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                if(!data.act){
                    console.warn("ws no action specified");
                    return;
                }
                switch(data.act){
                    case 'h':
                        window.location.reload();
                        break;
                    case 's':
                        const assets = document.querySelectorAll('link[rel="stylesheet"]:not([data-no-hot]), img:not([data-no-hot])');
                        assets.forEach(el => {
                            const attr = el.tagName === 'LINK' ? 'href' : 'src';
                            const url = new URL(el[attr], window.location.href);
                            url.searchParams.set('hot', Date.now());
                            el[attr] = url.href;
                        });
                        break;
                    default:
                        const targetElement = document.querySelector(data.target);
                        if (!targetElement) return;
                        switch(data.act){
                            case wsAction.APPEND:
                                targetElement.insertAdjacentHTML('beforeend', data.html);
                                break;

                            case wsAction.PREPEND:
                                targetElement.insertAdjacentHTML('afterbegin', data.html);
                                break;

                            case wsAction.REPLACE:
                                targetElement.innerHTML = data.html;
                                break;
                            default:
                                console.warn("ws unknown action received:", data.action);
                        }
                }
            } catch (err) {
                console.error("ws failed to parse message:", err);
            }
        };

        this.socket.onerror = (error) => {
            console.error("ws", error);
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
WS.connect();