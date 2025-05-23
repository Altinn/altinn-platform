import http from 'k6/http';
import { generateAccessToken } from '../maskinporten/maskinporten.js';

const BASE_URL = 'https://testdata.api.skatteetaten.no';
const DEFAULT_SCOPE = 'skatteetaten:testnorge/testdata.read';

/**
 * Sends a KQL search query to the Tenor testdata API.
 *
 * @param {string} encodedKql - Full URL-encoded KQL string (e.g. 'revisorer%3A*+and+organisasjonsnummer%3A312939053')
 * @param {string} [path='/api/testnorge/v2/soek/brreg-er-fr'] - Optional API path
 * @returns {object|null} - Parsed JSON response or null
 */
export function searchTenor({
  query,
  queryIsEncoded = true,
  path = '/api/testnorge/v2/soek/brreg-er-fr',
}) {
  if (typeof query !== 'string' || query.trim() === '') {
    throw new Error('searchTenor requires a non-empty query string');
  }

  const encodedQuery = queryIsEncoded ? query : encodeURIComponent(query);
  const token = generateAccessToken(DEFAULT_SCOPE);
  const url = `${BASE_URL}${path}?kql=${encodedQuery}&vis=tenorMetadata&antall=1`;

  const headers = {
    Authorization: `Bearer ${token}`,
    Accept: 'application/json',
  };

  let res;
  try {
    res = http.get(url, { headers });
  } catch (err) {
    console.error('HTTP request failed:', JSON.stringify(err, null, 2));
    return null;
  }

  if (!res || res.status !== 200) {
    console.error(`Ugyldig respons: ${res && res.status}`);
    console.error('Brukt URL:', url);
    return null;
  }

  return res.json();
}
