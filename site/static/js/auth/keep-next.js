const Init = (nextUri) =>{
    if (nextUri !== '/') {
    document.querySelectorAll('.keep-next').forEach(link => {
        try {
            const url = new URL(link.href, window.location.origin);
            url.searchParams.set('next', nextUri);
            link.href = url.toString();
        } catch (e) {
            console.error("Failed to append next param:", e);
        }
    });
}
};
export default Init;