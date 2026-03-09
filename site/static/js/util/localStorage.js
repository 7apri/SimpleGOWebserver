/**
 * Retrieves data from localStorage with an optional custom refresh logic.
 * @async
 * @param {string} key - The localStorage key.
 * @param {Function} callback - Async function to fetch fresh data.
 * @param {number} expiryMs - Default expiry time in milliseconds.
 * @param {Function} [refreshCondition] - Optional. A function that receives the
 * cached object {timestamp, data} and should return true if the cache is still valid.
 * @returns {Promise<any|null>} The cached or fresh data.
 */
const getLocalStorageTimestamp = async (key, callback, expiryMs, refreshCondition = null) => {
    const rawSaved = localStorage.getItem(key);
    let parsed = null;

    try {
        if (rawSaved) {
            parsed = JSON.parse(rawSaved);
            
            if(expiryMs === null || expiryMs === undefined) return parsed.data;
            
            const isFresh =  (Date.now() - parsed.timestamp) < expiryMs && (refreshCondition !== null ? refreshCondition?.(parsed) : true);

            if (isFresh) return parsed.data;
        }

        const freshData = await callback();
        if (freshData) saveWithTimestamp(key, freshData);
        return freshData;

    } catch (error) {
        console.error(`Error for key "${key}":`, error);;
        return parsed ? parsed.data : null;
    }
};

/**
 * Wraps data with a timestamp and saves it to localStorage.
 * @param {string} key - The localStorage key.
 * @param {any} data - The data to save (Object, Array, or String).
 */
const saveWithTimestamp = (key, data) => {
    const wrapper = {
        timestamp: Date.now(),
        data: data
    };
    
    localStorage.setItem(key, JSON.stringify(wrapper));
};

/**
 * Retrieves only the timestamp from a cached localStorage item.
 * * @param {string} key - The localStorage key to look up.
 * @returns {number|null} The Unix timestamp (ms) or null if the key doesn't exist or is invalid.
 */
const getTimestamp = (key) => {
    try{
        return JSON.parse(localStorage.getItem(key)).timestamp
    }
    catch(err) {
        console.error("Error[ " + err +  " ] getting timestamp for key: " + key);
        return null;
    }
};

export {saveWithTimestamp, getTimestamp};
export default getLocalStorageTimestamp;
