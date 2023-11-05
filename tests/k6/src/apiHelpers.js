/**
 * Build a string in a format of query param to the endpoint
 * @param {*} queryparams a json object with key as query name and value as query value
 * @example {"key1": "value1", "key2": "value2"}
 * @returns string a string like key1=value&key2=value2
 */
export function generateQueryParamString(queryparams) {
  var query = "?";
  Object.keys(queryparams).forEach(function (key) {
    if (Array.isArray(queryparams[key])) {
      queryparams[key].forEach((value) => {
        query += key + "=" + value + "&";
      });
    } else {
      query += key + "=" + queryparams[key] + "&";
    }
  });
  query = query.slice(0, -1);
  return query;
}