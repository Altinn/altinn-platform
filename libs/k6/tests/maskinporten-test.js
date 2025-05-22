import { generateAccessToken } from '../src/maskinporten/maskinporten.js';
import check from 'k6/http';

export default function () {
  const scopes =
    'skatteetaten:testnorge/testdata.read skatteetaten:testnorge/testdata.write';
  const token = generateAccessToken(scopes);
  check(token, { 'Token is not empty': (t) => !!t && t.length > 0 });
}
