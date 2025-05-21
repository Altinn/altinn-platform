/**
 * Converts a JSON object into a query parameter string for an API endpoint.
 * Handles both single values and arrays as query values.
 *
 * @param {Object} queryparams - A JSON object representing query parameters, where the key is the parameter name, and the value is the parameter value. 
 *                                If the value is an array, multiple key-value pairs are generated.
 * @example 
 * // Input
 * const queryparams = {
 *   key1: "value1",
 *   key2: "value2",
 *   key3: ["value3", "value4"]
 * };
 * 
 * // Output
 * // "?key1=value1&key2=value2&key3=value3&key3=value4"
 *
 * @returns {string} A properly formatted query parameter string, starting with a "?" and containing key-value pairs joined by "&".
 *                   For array values, the key is repeated for each value in the array.
 */
export function buildQueryParametersForEndpoint(queryparams) {
    let query = "?";

    Object.keys(queryparams).forEach((key) => {
        if (Array.isArray(queryparams[key])) {
            queryparams[key].forEach((value) => {
                query += `${key}=${value}&`;
            });
        } else {
            query += `${key}=${queryparams[key]}&`;
        }
    });

    query = query.slice(0, -1);

    return query;
}

/**
 * Build a header object with Bearer token
 * @param {string} token - The Bearer token
 * @returns {Object} A header object with Authorization set to Bearer token
 */
export function buildHeaderWithBearer(token) {
    return {
        headers: {
            Authorization: `Bearer ${token}`
        }
    };
}

/**
 * Build a header object with Basic token
 * @param {string} token - The Basic token
 * @returns {Object} A header object with Authorization set to Basic token
 */
export function buildHeaderWithBasic(token) {
    return {
        headers: {
            Authorization: `Basic ${token}`
        }
    };
}

/**
 * Build a header object with Bearer token and Content-Type set to application/json
 * @param {string} token - The Bearer token
 * @returns {Object} A header object with Authorization set to Bearer token and Content-Type set to application/json
 */
export function buildHeaderWithBearerAndContentType(token) {
    return {
        headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json"
        }
    };
}

/**
 * Build a header object with specified Content-Type
 * @param {string} contentType - The Content-Type value
 * @returns {Object} A header object with Content-Type set to the specified value
 */
export function buildHeaderWithContentType(contentType) {
    return {
        headers: {
            "Content-Type": contentType
        }
    };
}