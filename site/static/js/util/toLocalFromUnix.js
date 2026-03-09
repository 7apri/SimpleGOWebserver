/**
 * Converts a Unix timestamp to a localized time string 
 * using the specified timezone from the weather data.
 * * @param {number} unixTime - The Unix timestamp (in seconds/ms).
 * @param {string} timezone - The IANA timezone string (e.g., "Europe/Prague").
 * @param {Object} options - Options for toLocaleTimeString (e.g., {hour: '2-digit'}).
 * @param {string} locale - The locale string (e.g., "cs-CZ" or "en-GB").
 * @returns {string} The localized time string.
 */
export const getLocaleTimeString = (unixTime, timezone, options = {}, locale = "cs-CZ") => {
    const isMs = unixTime > 10000000000;
    const date = new Date(isMs ? unixTime : unixTime * 1000);

    const finalOptions = {
        ...options,
        timeZone: timezone
    };

    return date.toLocaleTimeString(locale, finalOptions);
};

/**
 * Converts a Unix timestamp to a localized weekday string.
 *
 * @param {number} unixTime - The Unix timestamp (in seconds).
 * @param {string} timeZone - The IANA timezone string (e.g., "Europe/Prague").
 * @param {string} [locale="cs-CZ"] - The locale string (e.g., "cs-CZ" or "en-GB").
 * @param {object} options - Options for toLocaleDateString (by default { weekday: 'short'}).
 * @returns {string} The localized short weekday string (e.g., "Po", "Út").
 */
export const getLocaleDateString = (unixTime, timeZone, options = {}, locale = "cs-CZ") => {
    const date = new Date(unixTime * 1000);

    const finalOptions = Object.keys(options).length === 0 ?{
        weekday: "short", 
    } : options;
    finalOptions.timeZone = timeZone;

    // Use toLocaleDateString, which is better suited for date parts than toLocaleTimeString
    return date.toLocaleDateString(locale, finalOptions);
};