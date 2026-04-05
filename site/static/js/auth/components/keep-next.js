const Init = (nextUrl) =>{
    if (nextUrl !== '/') {
        document.querySelectorAll('a[data-keep-next]').forEach(link => {
            try {
                const url = new URL(link.href, window.location.origin);
                url.searchParams.set('next', nextUrl);
                link.href = url.toString();
            } catch (e) {
                console.error(`failed to append next param on ${link} because: ${e}`);
            }
        });
    }
};
export default Init;