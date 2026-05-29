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
 * @param {Page}   page
*/
const RegisterPath = (path, page) =>{
    pages.set(path,page);
}

let selectedPage = "";

const SwitchPath = () => {
    pages.get(selectedPage)?.exit();

    const segments = window.location.pathname.split('/').filter(Boolean);
    
    while (segments.length >= 0) {
        const currentPath = "/" + segments.join('/');
        
        const exactPage = pages.get(currentPath);
        const wildcardPage = pages.get(currentPath === "/" ? "/*" : currentPath + "/*");
        const page = exactPage || wildcardPage;
        
        if (page) {
            page.enter();
            selectedPage = exactPage ? currentPath : (currentPath === "/" ? "/*" : currentPath + "/*");
            return; 
        }

        if (segments.length === 0) break;
        
        segments.pop();
    }

    const rootFallback = pages.get("/*");
    if (rootFallback) {
        rootFallback.enter();
        selectedPage = "/*";
    }
};
SwitchPath();

const originalPushState = history.pushState;
history.pushState = function(...args) {
    originalPushState.apply(this, args);
    window.dispatchEvent(new Event('locationchange'));
};
window.addEventListener('popstate', SwitchPath);
window.addEventListener('locationchange', SwitchPath);

export {RegisterPath};