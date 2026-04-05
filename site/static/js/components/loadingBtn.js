const ResetLoadEl = (el) => {
    el.disabled = false;
    el.classList.remove('loading');
}
const SetLoadingEl = (el) => {
    el.disabled = true;
    el.classList.add('loading');
}

window.addEventListener('pageshow', (event) => {
    console.log("hi");
    if (event.persisted) {
        document.querySelectorAll('.loading').forEach(ResetLoadEl);
    }
});

export {ResetLoadEl, SetLoadingEl};