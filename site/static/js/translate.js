window.tr = function(key, placeholders = {}, count = null) {
    let translation = (window.I18N_CACHE || {})[key];
    if (!translation) return `!?${key}?!`;

    if (count !== null && translation.includes('|')) {
        const parts = translation.split('|');
        translation = (count === 1) ? parts[0] : parts[1];
        placeholders['n'] = count; 
    }

    for (const [k, v] of Object.entries(placeholders)) {
        translation = translation.replaceAll(`{${k}}`, v);
    }
    return translation;
};