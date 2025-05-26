import http from 'k6/http';
import { check } from 'k6';
import encoding from 'k6/encoding';
import * as config from '../config.js';
import { stopIterationOnFail } from '../../common/errorhandler.js';
import { buildHeaderWithContentType } from '../../common/apiHelpers.js';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import KJUR from 'https://unpkg.com/jsrsasign@10.8.6/lib/jsrsasign.js';

const machineportenKid = __ENV.MACHINEPORTEN_KID;
const encodedJwk = __ENV.ENCODED_JWK;
const machineportenClientId = __ENV.MACHINEPORTEN_CLIENT_ID;

/**
 * Generates an access token from Maskinporten using a JWT bearer grant.
 *
 * This function performs the following steps:
 * 1. Validates required  variables (`__ENV.MACHINEPORTEN_KID`, `__ENV.ENCODED_JWK`, `__ENV.MACHINEPORTEN_CLIENT_ID`).
 * 2. Creates a signed JWT grant for the provided scopes.
 * 3. Sends a token request to Maskinporten.
 * 4. Validates the response and extracts the access token.
 *
 * @param {string} scopes - A space-separated string of scopes (e.g., "scope1 scope2").
 * @returns {string} The access token returned from Maskinporten.
 *
 * @throws Will stop test iteration if any required environment variable is missing or if the token request fails.
 */

// eslint-disable-next-line no-undef
const tokenCache = new Map(); // key: scopes, value: { token, expiresAt }

function createJwtGrant(scopes) {
  const header = {
    alg: 'RS256',
    typ: 'JWT',
    kid: machineportenKid,
  };

  const now = Math.floor(Date.now() / 1000);

  const payload = {
    aud: config.maskinporten.audience,
    scope: scopes,
    iss: machineportenClientId,
    iat: now,
    exp: now + 120,
    jti: uuidv4(),
  };

  const signedJWT = KJUR.jws.JWS.sign(
    'RS256',
    header,
    payload,
    JSON.parse(encoding.b64decode(encodedJwk, 'std', 's')),
  );

  return signedJWT;
}

function decodeJwtPayload(jwt) {
  const base64 = jwt
    .split('.')[1]
    .replace(/-/g, '+')
    .replace(/_/g, '/')
    .padEnd(4 * Math.ceil(jwt.split('.')[1].length / 4), '=');
  return JSON.parse(encoding.b64decode(base64, 'std', 's'));
}
