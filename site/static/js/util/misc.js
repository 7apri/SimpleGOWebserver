/**
 * Capitalizes the first letter of a string.
 * * @param {string} str - The string to be capitalized.
 * @returns {string} The string with the first letter in uppercase.
 */
const capitalize = (str) => str.charAt(0).toUpperCase() + str.slice(1);

/**
 * Formats a year, month, and day into a date string (YYYY-MM-DD).
 * * @param {number|string} year - The year.
 * @param {number|string} month - The month (1-12).
 * @param {number|string} day - The day (1-31).
 * @returns {string} Formatted date string with padded zeros.
 */
const toDateString = (year, month, day) => 
    `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;

/**
 * Returns the correct Czech grammatical form of the word "degree" (stupeň) based on the number.
 * * @param {number} n - The numeric value.
 * @returns {string} The correct Czech word form: "stupeň", "stupně", or "stupňů".
 */
const stupne = (n) => {
    const absN = Math.abs(n);
    if (absN === 1) return "stupeň";
    if (absN >= 2 && absN <= 4) return "stupně";
    return "stupňů";
};

export { capitalize, toDateString, stupne };