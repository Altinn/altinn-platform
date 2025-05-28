import http from 'k6/http';
import { check } from 'k6';
import { generateAccessToken } from '../maskinporten/maskinporten.js';

const BASE_URL = 'https://testdata.api.skatteetaten.no';
const DEFAULT_SCOPE = 'skatteetaten:testnorge/testdata.read';

/**
 * Henter et organisasjonsnummer for en gitt rolle fra Tenor-testdata
 *
 * @param {Object} options
 * @param {string} options.query - Ferdig URL-encoded query (f.eks. 'revisorer%3A*')
 * @param {string} [options.role='REVI'] - Rollekode å filtrere på
 * @param {string} [options.path='/api/testnorge/v2/soek/brreg-er-fr'] - API-path
 * @returns {string|null} - Første organisasjonsnummer funnet for rollen, eller null
 */
export function hentOrgnummerForRolle({
  query,
  role = 'REVI',
  path = '/api/testnorge/v2/soek/brreg-er-fr',
}) {
  if (typeof query !== 'string' || query.trim() === '') {
    throw new Error('Parameter "query" must be a non-empty URL-encoded string');
  }

  const token = generateAccessToken(DEFAULT_SCOPE);
  const url = `${BASE_URL}${path}?kql=${query}&vis=tenorMetadata&antall=1`;

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
    console.error('URL brukt:', url);
    return null;
  }

  const data = res.json();
  const dokument = data?.dokumentListe?.[0];
  if (!dokument) {
    console.warn('Ingen dokumenter funnet');
    return null;
  }

  const kildedata = JSON.parse(dokument.tenorMetadata.kildedata);
  const rollegrupper = kildedata.rollegrupper || [];

  for (const gruppe of rollegrupper) {
    if (gruppe.type?.kode === role) {
      for (const rolle of gruppe.roller || []) {
        const raw = JSON.stringify(rolle.virksomhet || {});
        const match = raw.match(/\b\d{9}\b/);
        if (match) {
          const orgnummer = match[0];
          check(orgnummer, {
            [`Fant 9-sifret orgnummer for ${role}`]: (v) => /^\d{9}$/.test(v),
          });
          return orgnummer;
        }
      }
    }
  }

  console.warn(`Ingen organisasjonsnummer funnet for rolle: ${role}`);
  return null;
}
