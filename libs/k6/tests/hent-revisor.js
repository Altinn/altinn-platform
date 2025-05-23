import { generateAccessToken } from '../src/maskinporten/maskinporten.js';
import http from 'k6/http';
import { check } from 'k6';
import { searchTenor } from '../src/tenor/search.js';
import {
  hentOrgnummerForRolle,
  hentFoedselsnummerForDagligLeder,
} from '../src/tenor/extract.js';

//tenorMetadata = kildedata
//antall=1

const url =
  // eslint-disable-next-line max-len
  'https://testdata.api.skatteetaten.no/api/testnorge/v2/soek/brreg-er-fr?kql=revisorer%3A*&vis=tenorMetadata&antall=1';
//   ,visningnavn,tenorMetadata

function hentTestdata() {
  const scopes = 'skatteetaten:testnorge/testdata.read';
  const token = generateAccessToken(scopes);

  const headers = {
    Authorization: `Bearer ${token}`,
  };

  let res;

  console.log(url);

  try {
    res = http.get(url, { headers });
  } catch (err) {
    console.error('HTTP request failed:', JSON.stringify(err, null, 2));
    return;
  }

  if (!res) {
    console.error('Response is null');
    return;
  }

  const data = res.json();

  const dokument = data.dokumentListe[0];
  const kildedata = JSON.parse(dokument.tenorMetadata.kildedata);
  const rollegrupper = kildedata.rollegrupper;

  let orgnummer = null;

  for (const gruppe of rollegrupper) {
    if (gruppe.type?.kode === 'REVI') {
      const rolle = gruppe.roller?.find(
        (r) => r.virksomhet?.organisasjonsnummer,
      );
      if (rolle) {
        orgnummer = rolle.virksomhet.organisasjonsnummer;
        break;
      }
    }
  }

  console.log('Organisasjonsnummer for REVI:', orgnummer);

  check(orgnummer, {
    'Fant 9-sifret orgnummer': (v) => /^\d{9}$/.test(v),
  });
}

export default function () {
  const json = searchTenor({ query: 'revisorer%3A*' }); // Already encoded
  const orgnummer = hentOrgnummerForRolle(json);
  console.log('Funnet orgnummer:', orgnummer);

  //slå opp på orgnummeret
  //testdata.api.skatteetaten.no/api/testnorge/v2/soek/brreg-er-fr?kql=revisorer%3A*+and+organisasjonsnummer%3A312939053
  const orgSearch = searchTenor({
    query: `organisasjonsnummer:${orgnummer}`,
    queryIsEncoded: false,
  });

  var resp = hentFoedselsnummerForDagligLeder(orgSearch);
  console.log(resp);
}
