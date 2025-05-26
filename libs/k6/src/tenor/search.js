import http from 'k6/http';
import { generateAccessToken } from '../maskinporten/maskinporten.js';

const BASE_URL = 'https://testdata.api.skatteetaten.no';
const DEFAULT_SCOPE = 'skatteetaten:testnorge/testdata.read';

/**
 * Sends a KQL search query to the Tenor testdata API.
 *
 * @param {object} options
 * @param {string} options.query - Raw or encoded KQL query string
 * @param {string} [options.path='/api/testnorge/v2/soek/brreg-er-fr'] - Optional API path
 * @param {number} [options.antall=1] - Number of results to request
 * @param {boolean} [options.includeTenorMetadata=true] - Whether to include vis=tenorMetadata in query
 * @returns {object|null} - Parsed JSON response or null
 */
export function searchTenor({
  query,
  path = '/api/testnorge/v2/soek/brreg-er-fr',
  antall = 1,
  includeTenorMetadata = true,
}) {
  if (typeof query !== 'string' || query.trim() === '') {
    throw new Error('searchTenor requires a non-empty query string');
  }

  const token = generateAccessToken(DEFAULT_SCOPE);

  let url = `${BASE_URL}${path}?kql=${query}`;
  if (includeTenorMetadata) {
    url += '&vis=tenorMetadata';
  }
  url += `&antall=${antall}`;

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
