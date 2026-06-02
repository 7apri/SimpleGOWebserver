import { RegisterPath } from "./router.js";

class RelativeTime extends HTMLElement {
    connectedCallback() {
        const mode = this.getAttribute('mode') || 'relative';
        const date = new Date(this.getAttribute('datetime'));
        if (mode === 'absolute') {
            this.textContent = date.toLocaleDateString(undefined, { 
                year: 'numeric', month: 'long', day: 'numeric' 
            });
        } else {
            this.textContent = this.formatRelative(date);
            this.interval = setInterval(() => this.textContent = this.formatRelative(date), 60000);
        }
    }

    disconnectedCallback() {
        clearInterval(this.interval);
    }

    formatRelative(date) {
        const now = new Date();
        const diffInSeconds = Math.floor((now - date) / 1000);

        if (diffInSeconds < 60) return tr("just_now");
        
        const diffInMinutes = Math.floor(diffInSeconds / 60);
        if (diffInMinutes < 60) return `${diffInMinutes}m`;
        
        const diffInHours = Math.floor(diffInMinutes / 60);
        if (diffInHours < 24) return `${diffInHours}h`;
        
        const diffInDays = Math.floor(diffInHours / 24);
        if (diffInDays < 7) return `${diffInDays}d`;
        
        return date.toLocaleDateString(); 
    }
}

customElements.define('relative-time', RelativeTime);

document.getElementById('side-left-info-more-btn')?.addEventListener("click", () => document.getElementById('side-left-info-more').classList.toggle('hide'))
document.addEventListener('htmx:configRequest', (event) => {
    if (window.location.pathname === event.detail.path) {
        event.preventDefault();
    }
});
const postInputResize = (e) =>{
    e.target.style.height = 'auto';
    e.target.style.height = e.target.scrollHeight + 'px';
}
const enterHome = () =>{
    document.querySelector(".post-input")?.addEventListener("input", postInputResize)
};
RegisterPath("/", {
    enter: enterHome,
    exit: () => console.log("exit home")
})

const enterChat = () =>{
    document.getElementById("msg-input")?.addEventListener("input", postInputResize)
};
RegisterPath("/chats/*", {
    enter: enterChat,
    exit: () => console.log("exit chat")
})