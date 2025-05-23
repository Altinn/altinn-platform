import { check } from 'k6';

/**
 * Extracts the first organisasjonsnummer from a parsed Tenor response for a given role
 *
 * @param {object} responseJson - Response from `searchTenor`
 * @param {string} role - Rollekode (e.g. 'REVI')
 * @returns {string|null} - 9-digit organisasjonsnummer or null
 */
export function hentOrgnummerForRolle(responseJson, role = 'REVI') {
  const dokument = responseJson?.dokumentListe?.[0];
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

/**
 * Extracts the fødselsnummer for the DAGL (daglig leder) role
 *
 * @param {object} responseJson - Response from `searchTenor`
 * @returns {string|null} - Fødselsnummer or null
 */
export function hentFoedselsnummerForDagligLeder(responseJson) {
  const dokument = responseJson?.dokumentListe?.[0];
  if (!dokument) {
    console.log('Ingen dokumenter funnet');
    return null;
  }

  const kildedata = JSON.parse(dokument.tenorMetadata.kildedata);
  const rollegrupper = kildedata.rollegrupper || [];

  for (const gruppe of rollegrupper) {
    if (gruppe.type?.kode === 'DAGL') {
      for (const rolle of gruppe.roller || []) {
        const fnr = rolle.person?.foedselsnummer;
        if (fnr) {
          return fnr;
        }
      }
    }
  }

  console.log('Fødselsnummer for DAGL ikke funnet');
  return null;
}
