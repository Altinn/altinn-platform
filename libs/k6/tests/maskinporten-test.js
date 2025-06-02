import { generateAccessToken } from '../src/maskinporten/maskinporten.js';
import http from 'k6/http';

export const options = {
  insecureSkipTLSVerify: true,
};

// function generateAccessToken() {
//   const scopes =
//     'skatteetaten:testnorge/testdata.read skatteetaten:testnorge/testdata.write';
//   const token = generateAccessToken(scopes);
//   console.log(token);
// }

function hentTestdata() {
  const scopes = 'skatteetaten:testnorge/testdata.read';
  const token = generateAccessToken(scopes);
  console.log(token);

  const url =
    // eslint-disable-next-line max-len
    'https://testdata.api.skatteetaten.no/api/testnorge/v2/soek/brreg-er-fr?kql=organisasjonsform.kode:AS&highlight=true&vis=navn,visningnavn,tenorMetadata';
  const headers = {
    Authorization: `Bearer ${token}`,
  };

  console.log(`Calling: ${url}`);
  console.log(`Using token: ${token.slice(0, 30)}...`);

  let res;

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

  console.log('Response status:', res.status);
  console.log('Response body:', res.body);
}

export default function () {
  hentTestdata();
}
