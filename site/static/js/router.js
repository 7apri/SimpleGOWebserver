/**
 * @type {Map<String,Page>}
 */
const pages = new Map();

/**
 * @typedef {Object} Page
 * @property {function(): void} enter
 * @property {function(): void} exit
 */

/**
 * Registers a page into the router
 * @param {String} path 
 * @param {Page} page
 */
const RegisterPath = (path, page) => {
    pages.set(path, page);
}

let activePath = "";
let activePage = null;
let swapped = false;

const htmxWrapper = () => {
    if (!swapped && activePage && typeof activePage.enter === 'function') {
        swapped = true;
        activePage.enter();
    }
};

const SwitchPath = () => {
    if (activePage) {
        if (typeof activePage.exit === 'function') activePage.exit();
        document.removeEventListener("htmx:afterSwap", htmxWrapper);
    }

    const segments = window.location.pathname.split('/').filter(Boolean);
    let page = null;
    let matchedPath = "";
    let isFirst = true;
    while (segments.length >= 0) {
        const currentPath = "/" + segments.join('/');
        
        const exactPage = pages.get(currentPath);
        const wildcardPage = pages.get(currentPath === "/" ? "/*" : currentPath + "/*");
        
        if (exactPage) {
            if(currentPath == "/" && !isFirst){
            } else {
                page = exactPage;
                matchedPath = currentPath;
                break;
            }
        }
        if (wildcardPage) {
            page = wildcardPage;
            matchedPath = currentPath === "/" ? "/*" : currentPath + "/*";
            break;
        }

        if (segments.length === 0) break;
        segments.pop();
        isFirst = false;
    }

    if (!page) {
        page = pages.get("/*");
        matchedPath = "/*";
    }

    if (page) {
        activePath = matchedPath;
        activePage = page;
        swapped = false;
        document.addEventListener("htmx:afterSwap", htmxWrapper);
    } else {
        selectedPage = "";
    }
};

document.addEventListener('DOMContentLoaded',() =>{
    SwitchPath();
    activePage?.enter();
});

const originalPushState = history.pushState;
history.pushState = function(...args) {
    originalPushState.apply(this, args);
    window.dispatchEvent(new Event('locationchange'));
};

window.addEventListener('popstate', SwitchPath);
window.addEventListener('locationchange', SwitchPath);

export { RegisterPath };